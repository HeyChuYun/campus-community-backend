package repository

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hfjj/backend/passport-service/internal/config"
)

const behaviorVerifyProviderGeeTestV4 = "geetest_v4"

type BehaviorVerifyPayload struct {
	LotNumber     string
	CaptchaOutput string
	PassToken     string
	GenTime       string
}

type BehaviorVerifyPublicConfig struct {
	Enabled   bool
	Provider  string
	CaptchaID string
}

type BehaviorVerifyResult struct {
	Passed bool
	Reason string
}

type BehaviorVerifier interface {
	PublicConfig() BehaviorVerifyPublicConfig
	Verify(ctx context.Context, payload BehaviorVerifyPayload) (BehaviorVerifyResult, error)
}

type noopBehaviorVerifier struct{}

type geeTestVerifier struct {
	client      *http.Client
	captchaID   string
	captchaKey  string
	validateURL string
}

type geeTestValidateResponse struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

func NewBehaviorVerifier(cfg config.GeeTestConfig) BehaviorVerifier {
	if !cfg.Enabled {
		return noopBehaviorVerifier{}
	}

	validateURL := strings.TrimSpace(cfg.ValidateURL)
	if validateURL == "" {
		validateURL = "https://gcaptcha4.geetest.com/validate"
	}

	timeoutMillis := cfg.TimeoutMillis
	if timeoutMillis <= 0 {
		timeoutMillis = 5000
	}

	return &geeTestVerifier{
		client: &http.Client{
			Timeout: time.Duration(timeoutMillis) * time.Millisecond,
		},
		captchaID:   strings.TrimSpace(cfg.CaptchaID),
		captchaKey:  strings.TrimSpace(cfg.CaptchaKey),
		validateURL: validateURL,
	}
}

func (noopBehaviorVerifier) PublicConfig() BehaviorVerifyPublicConfig {
	return BehaviorVerifyPublicConfig{
		Enabled:   false,
		Provider:  behaviorVerifyProviderGeeTestV4,
		CaptchaID: "",
	}
}

func (noopBehaviorVerifier) Verify(context.Context, BehaviorVerifyPayload) (BehaviorVerifyResult, error) {
	return BehaviorVerifyResult{Passed: true, Reason: "disabled"}, nil
}

func (v *geeTestVerifier) PublicConfig() BehaviorVerifyPublicConfig {
	return BehaviorVerifyPublicConfig{
		Enabled:   true,
		Provider:  behaviorVerifyProviderGeeTestV4,
		CaptchaID: v.captchaID,
	}
}

func (v *geeTestVerifier) Verify(ctx context.Context, payload BehaviorVerifyPayload) (BehaviorVerifyResult, error) {
	// 极验启用后，服务端必须拿到完整的验证结果，缺任何字段都直接拒绝。
	if strings.TrimSpace(v.captchaID) == "" || strings.TrimSpace(v.captchaKey) == "" {
		return BehaviorVerifyResult{}, errors.New("geetest captcha_id or captcha_key is empty")
	}
	if strings.TrimSpace(payload.LotNumber) == "" ||
		strings.TrimSpace(payload.CaptchaOutput) == "" ||
		strings.TrimSpace(payload.PassToken) == "" ||
		strings.TrimSpace(payload.GenTime) == "" {
		return BehaviorVerifyResult{Passed: false, Reason: "missing geetest behavior verify fields"}, nil
	}

	form := url.Values{}
	form.Set("lot_number", strings.TrimSpace(payload.LotNumber))
	form.Set("captcha_output", strings.TrimSpace(payload.CaptchaOutput))
	form.Set("pass_token", strings.TrimSpace(payload.PassToken))
	form.Set("gen_time", strings.TrimSpace(payload.GenTime))
	form.Set("sign_token", buildGeeTestSignToken(v.captchaKey, payload.LotNumber))

	validateURL, err := url.Parse(v.validateURL)
	if err != nil {
		return BehaviorVerifyResult{}, err
	}
	query := validateURL.Query()
	query.Set("captcha_id", v.captchaID)
	validateURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		validateURL.String(),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return BehaviorVerifyResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return BehaviorVerifyResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return BehaviorVerifyResult{}, errors.New("geetest validate http status: " + resp.Status)
	}

	var result geeTestValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BehaviorVerifyResult{}, err
	}

	if strings.EqualFold(strings.TrimSpace(result.Result), "success") ||
		strings.EqualFold(strings.TrimSpace(result.Status), "success") {
		return BehaviorVerifyResult{Passed: true, Reason: "success"}, nil
	}

	reason := strings.TrimSpace(result.Reason)
	if reason == "" {
		reason = strings.TrimSpace(result.Msg)
	}
	if reason == "" {
		reason = "behavior verify failed"
	}
	return BehaviorVerifyResult{Passed: false, Reason: reason}, nil
}

func buildGeeTestSignToken(captchaKey, lotNumber string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(captchaKey)))
	mac.Write([]byte(strings.TrimSpace(lotNumber)))
	return hex.EncodeToString(mac.Sum(nil))
}
