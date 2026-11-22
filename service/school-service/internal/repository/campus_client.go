package repository

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/captcha-solver-service/captchasolverservice"
	"hfjj/backend/proto/captchapb"
	"hfjj/backend/proto/schoolpb"
	"hfjj/backend/school-service/internal/config"
	"hfjj/backend/school-service/internal/types"
)

// CampusError wraps upstream campus-system errors into service-level error codes.
type CampusError struct {
	Code    schoolpb.SchoolCode
	Message string
}

func (e *CampusError) Error() string {
	return e.Message
}

// IsTicketExchangeServerError identifies 5xx failures returned by the CAS ticket exchange step.
func IsTicketExchangeServerError(err error) bool {
	campusErr, ok := err.(*CampusError)
	if !ok {
		return false
	}

	return campusErr.Code == schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR &&
		strings.Contains(campusErr.Message, "鎹㈠彇 service ticket 澶辫触: http 500")
}

// CampusClient handles interactions with the campus SSO and academic systems.
type CampusClient struct {
	config     config.CampusConfig
	captchaRPC captchasolverservice.CaptchaSolverService
}

// AuthorizeResult contains the credentials returned by the campus authorization flow.
type AuthorizeResult struct {
	StudentNo   string
	StudentName string
	TGT         string
	Ticket      string
}

func NewCampusClient(cfg config.CampusConfig, captchaRPC captchasolverservice.CaptchaSolverService) *CampusClient {
	if cfg.VPNBaseURL == "" {
		cfg.VPNBaseURL = "https://vpn.example.edu:8443"
	}
	if cfg.EnlinkSSOLoginURL == "" {
		cfg.EnlinkSSOLoginURL = cfg.VPNBaseURL + "/enlink/sso/login"
	}
	if cfg.TargetServiceURL == "" {
		cfg.TargetServiceURL = "http://campus.example.edu/sso/lyiotlogin"
	}
	if cfg.CASRoot == "" {
		cfg.CASRoot = cfg.VPNBaseURL + "/https/webvpnc380013bd70394826ced621b793ef23ac8d91eada09f2baa26eec6b87634ed25/lyuapServer"
	}
	if cfg.JWBaseURL == "" {
		cfg.JWBaseURL = cfg.VPNBaseURL + "/http/webvpn1d1800074c8dbd59fe8bc5cfd2e64912"
	}
	if cfg.PersonalInfoURL == "" {
		cfg.PersonalInfoURL = cfg.JWBaseURL + "/xsxxxggl/xsgrxxwh_cxXsgrxx.html?gnmkdm=N100801&layout=default"
	}
	if cfg.ScheduleIndexURL == "" {
		cfg.ScheduleIndexURL = cfg.JWBaseURL + "/kbcx/xskbcx_cxXskbcxIndex.html?gnmkdm=N2151&layout=default"
	}
	if cfg.ScheduleDataURL == "" {
		cfg.ScheduleDataURL = cfg.JWBaseURL + "/kbcx/xskbcx_cxXsgrkb.html?gnmkdm=N2151"
	}
	if cfg.LoginToken == "" {
		cfg.LoginToken = "loginToken"
	}
	if cfg.LoginUserToken == "" {
		cfg.LoginUserToken = "CHANGE_ME_LOGIN_USER_TOKEN"
	}
	if cfg.RSAExponentHex == "" {
		cfg.RSAExponentHex = "010001"
	}
	if cfg.RSAModulusHex == "" {
		cfg.RSAModulusHex = ""
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &CampusClient{
		config:     cfg,
		captchaRPC: captchaRPC,
	}
}

// PreAuthorize 打开校园认证链路并获取图形验证码。
func (c *CampusClient) PreAuthorize(ctx context.Context) (types.PreAuthSession, error) {
	client, jar, err := c.newHTTPClient(nil, true)
	if err != nil {
		return types.PreAuthSession{}, err
	}
	if err := c.bootstrapSession(ctx, client); err != nil {
		return types.PreAuthSession{}, err
	}

	body, status, _, err := c.doGet(ctx, client, c.captchaURL(), map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Referer":          c.casLoginURL(),
		"User-Agent":       c.config.UserAgent,
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return types.PreAuthSession{}, err
	}
	if status != http.StatusOK {
		return types.PreAuthSession{}, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("获取验证码失败: http %d", status),
		}
	}

	var payload struct {
		UID     string `json:"uid"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return types.PreAuthSession{}, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("解析验证码响应失败: %v", err),
		}
	}
	if strings.TrimSpace(payload.UID) == "" || strings.TrimSpace(payload.Content) == "" {
		return types.PreAuthSession{}, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: "captcha response missing required fields",
		}
	}

	session := types.PreAuthSession{
		AuthSessionID: randomID("auth"),
		CaptchaUID:    payload.UID,
		CaptchaBase64: payload.Content,
		Cookies:       exportCookies(jar.Cookies(c.baseURL())),
		CreatedAt:     time.Now(),
	}
	return session, nil
}

// Authorize 提交学号、校园密码和验证码，完成校园登录。
func (c *CampusClient) Authorize(ctx context.Context, session types.PreAuthSession, studentNo, schoolPassword, captchaCode string) (*AuthorizeResult, error) {
	client, jar, err := c.newHTTPClient(nil, true)
	if err != nil {
		return nil, err
	}
	jar.SetCookies(c.baseURL(), restoreCookies(session.Cookies))

	encryptedPassword, err := c.encryptPassword(schoolPassword)
	if err != nil {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_INTERNAL_ERROR,
			Message: fmt.Sprintf("密码加密失败: %v", err),
		}
	}

	form := url.Values{}
	form.Set("username", studentNo)
	form.Set("password", encryptedPassword)
	form.Set("service", c.config.TargetServiceURL)
	form.Set("loginType", "")
	form.Set("id", session.CaptchaUID)
	form.Set("code", captchaCode)

	body, status, _, err := c.doPostForm(
		ctx,
		client,
		c.ticketsURL(),
		form,
		c.ajaxHeaders(jar, c.casLoginURL()),
	)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("学校登录失败: http %d", status),
		}
	}

	result, err := parseLoginResponse(body)
	if err != nil {
		return nil, err
	}
	result.StudentNo = studentNo
	return result, nil
}

// ValidateSession 使用已有 TGT 试探性换票，判断学校侧登录态是否还有效。
func (c *CampusClient) ValidateSession(ctx context.Context, session types.SchoolSession) error {
	client, jar, err := c.newHTTPClient(nil, true)
	if err != nil {
		return err
	}
	c.ensureCASTGCCookie(jar, session.TGT)
	if err := c.bootstrapSession(ctx, client); err != nil {
		return err
	}

	ticket, err := c.exchangeServiceTicket(ctx, client, jar, session.TGT, c.config.TargetServiceURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(ticket) == "" {
		return &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_SESSION_EXPIRED,
			Message: "school TGT expired",
		}
	}
	return nil
}

// ReLogin 在学校侧 session 失效后，使用保存的学号密码重新登录。
func (c *CampusClient) ReLogin(ctx context.Context, studentNo, schoolPassword string) (*AuthorizeResult, error) {
	preAuth, err := c.PreAuthorize(ctx)
	if err != nil {
		return nil, err
	}

	captchaCode, err := c.solveCaptcha(ctx, preAuth.CaptchaBase64)
	if err != nil {
		return nil, err
	}

	return c.Authorize(ctx, preAuth, studentNo, schoolPassword, captchaCode)
}

// GetStudentProfile 使用学校侧 TGT 进入教务并抓取个人信息。
func (c *CampusClient) GetStudentProfile(ctx context.Context, session types.SchoolSession) (*types.StudentProfilePayload, error) {
	client, jar, err := c.newHTTPClient(nil, true)
	if err != nil {
		return nil, err
	}
	c.ensureCASTGCCookie(jar, session.TGT)
	if err := c.bootstrapSession(ctx, client); err != nil {
		return nil, err
	}

	body, status, finalURL, err := c.requestPersonalInfoPage(ctx, client, c.casLoginURL())
	if err != nil {
		return nil, err
	}
	htmlText := string(body)
	if status == http.StatusOK && c.isPersonalInfoHTML(htmlText) {
		_, _, _, _ = c.doGet(ctx, client, c.initMenuURL(), c.htmlHeaders(finalURL))
		return parseStudentProfile(htmlText), nil
	}

	callbackService := strings.TrimRight(c.config.VPNBaseURL, "/") + "/enlink/api/client/callback/cas"
	callbackTicket, err := c.exchangeServiceTicket(ctx, client, jar, session.TGT, callbackService)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(callbackTicket) == "" {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_SESSION_EXPIRED,
			Message: "学校 TGT 已失效，无法换取 callback ticket",
		}
	}

	noRedirectClient, _, err := c.newHTTPClient(jar, false)
	if err != nil {
		return nil, err
	}
	callbackURL := callbackService + "?ticket=" + url.QueryEscape(callbackTicket)
	if _, _, _, err = c.doGet(ctx, noRedirectClient, callbackURL, c.htmlHeaders(finalURL)); err != nil {
		return nil, err
	}

	entoken := c.decodeClientSID(jar)
	if entoken == "" {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: "未从 clientInfo cookie 解析到 Enlink sid",
		}
	}

	jwTicket, err := c.exchangeServiceTicket(ctx, client, jar, session.TGT, c.config.TargetServiceURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(jwTicket) == "" {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_SESSION_EXPIRED,
			Message: "学校 TGT 已失效，无法换取教务系统 ticket",
		}
	}

	lyiotURL := strings.TrimRight(c.config.JWBaseURL, "/") + "/sso/lyiotlogin?ticket=" + url.QueryEscape(jwTicket)
	authorizedURL := strings.TrimRight(c.config.VPNBaseURL, "/") +
		"/engateway/api/sso/authorized?redirect_url=" + url.QueryEscape(lyiotURL) +
		"&entoken=" + url.QueryEscape(entoken)

	body, _, finalURL, err = c.doGet(ctx, client, authorizedURL, c.htmlHeaders(callbackURL))
	if err != nil {
		return nil, err
	}

	htmlText, finalURL, err = c.walkHTMLSSO(ctx, client, string(body), finalURL, authorizedURL)
	if err != nil {
		return nil, err
	}

	_, _, menuURL, _ := c.doGet(ctx, client, c.initMenuURL(), c.htmlHeaders(finalURL))
	body, status, _, err = c.requestPersonalInfoPage(ctx, client, menuURL)
	if err != nil {
		return nil, err
	}
	htmlText = string(body)
	if status != http.StatusOK || !c.isPersonalInfoHTML(htmlText) {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: "failed to enter student profile page",
		}
	}

	return parseStudentProfile(htmlText), nil
}

// solveCaptcha 用于自动重登时识别验证码。
func (c *CampusClient) solveCaptcha(ctx context.Context, imageBase64 string) (string, error) {
	if strings.TrimSpace(c.config.CaptchaFixedCode) != "" {
		return strings.TrimSpace(c.config.CaptchaFixedCode), nil
	}
	if c.captchaRPC == nil {
		return "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REAUTH_REQUIRED,
			Message: "captcha solver not configured, re-authorization required",
		}
	}

	resp, err := c.captchaRPC.SolveCaptcha(ctx, &captchapb.SolveCaptchaRequest{
		ImageBase64: imageBase64,
	})
	if err != nil {
		return "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REAUTH_REQUIRED,
			Message: fmt.Sprintf("验证码识别服务请求失败: %v", err),
		}
	}
	if resp.GetBase().GetCode() != captchapb.CaptchaCode_CAPTCHA_CODE_OK {
		return "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REAUTH_REQUIRED,
			Message: strings.TrimSpace(resp.GetBase().GetMessage()),
		}
	}

	code := strings.TrimSpace(resp.GetCode())
	if code == "" {
		return "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REAUTH_REQUIRED,
			Message: "captcha solve failed, re-authorization required",
		}
	}
	return code, nil
}

// bootstrapSession 依次打开 VPN SSO 和 CAS 登录页，初始化必要 cookie。
func (c *CampusClient) bootstrapSession(ctx context.Context, client *http.Client) error {
	if _, _, _, err := c.doGet(ctx, client, c.config.EnlinkSSOLoginURL, c.htmlHeaders(c.config.VPNBaseURL)); err != nil {
		return err
	}
	if _, _, _, err := c.doGet(ctx, client, c.casLoginURL(), c.htmlHeaders(c.config.EnlinkSSOLoginURL)); err != nil {
		return err
	}
	return nil
}

// requestPersonalInfoPage 请求个人信息页面。
func (c *CampusClient) requestPersonalInfoPage(ctx context.Context, client *http.Client, referer string) ([]byte, int, string, error) {
	return c.doGet(ctx, client, c.config.PersonalInfoURL, c.htmlHeaders(referer))
}

// exchangeServiceTicket 使用 TGT 换取指定 service 的 service ticket。
func (c *CampusClient) exchangeServiceTicket(ctx context.Context, client *http.Client, jar *cookiejar.Jar, tgt, service string) (string, error) {
	form := url.Values{}
	form.Set("service", service)
	form.Set("loginToken", c.config.LoginToken)

	var (
		body   []byte
		status int
		err    error
	)

	for attempt := 1; attempt <= 3; attempt++ {
		body, status, _, err = c.doPostForm(
			ctx,
			client,
			c.ticketExchangeURL(tgt),
			form,
			c.ajaxHeaders(jar, c.casLoginURL()),
		)
		if err != nil {
			return "", err
		}
		if status < http.StatusInternalServerError {
			break
		}
		if attempt == 3 {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
		}
	}

	if status >= http.StatusBadRequest {
		return "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("换取 service ticket 失败: http %d", status),
		}
	}

	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", nil
	}

	var payload any
	if err = json.Unmarshal(body, &payload); err == nil {
		if data, ok := payload.(map[string]any); ok {
			if ticket := strings.TrimSpace(anyToString(data["ticket"])); ticket != "" {
				return ticket, nil
			}
			if ticket := strings.TrimSpace(anyToString(data["data"])); ticket != "" {
				return ticket, nil
			}
		}
	}

	return text, nil
}

// walkHTMLSSO 处理教务系统登录链路中的脚本跳转、meta refresh 和自动提交表单。
func (c *CampusClient) walkHTMLSSO(ctx context.Context, client *http.Client, htmlText, finalURL, referer string) (string, string, error) {
	lastHTML := htmlText
	lastURL := finalURL
	currentReferer := referer

	for step := 0; step < 8; step++ {
		redirectURL := extractJSRedirectURL(lastHTML, lastURL)
		if redirectURL == "" {
			redirectURL = extractMetaRefreshURL(lastHTML, lastURL)
		}
		if redirectURL != "" && redirectURL != lastURL {
			body, _, nextURL, err := c.doGet(ctx, client, redirectURL, c.htmlHeaders(currentReferer))
			if err != nil {
				return "", "", err
			}
			lastHTML = string(body)
			lastURL = nextURL
			currentReferer = nextURL
			continue
		}

		formURL, formMethod, formFields := parseAutoSubmitForm(lastHTML, lastURL)
		if formURL == "" {
			break
		}

		currentReferer = lastURL
		if strings.EqualFold(formMethod, http.MethodPost) {
			form := url.Values{}
			for key, value := range formFields {
				form.Set(key, value)
			}
			body, _, nextURL, err := c.doPostForm(ctx, client, formURL, form, c.htmlHeaders(currentReferer))
			if err != nil {
				return "", "", err
			}
			lastHTML = string(body)
			lastURL = nextURL
			continue
		}

		targetURL := formURL
		if len(formFields) > 0 {
			values := url.Values{}
			for key, value := range formFields {
				values.Set(key, value)
			}
			if strings.Contains(targetURL, "?") {
				targetURL += "&" + values.Encode()
			} else {
				targetURL += "?" + values.Encode()
			}
		}

		body, _, nextURL, err := c.doGet(ctx, client, targetURL, c.htmlHeaders(currentReferer))
		if err != nil {
			return "", "", err
		}
		lastHTML = string(body)
		lastURL = nextURL
	}

	return lastHTML, lastURL, nil
}

// decodeClientSID extracts the Enlink SID from the clientInfo cookie.
func (c *CampusClient) decodeClientSID(jar *cookiejar.Jar) string {
	raw := c.getCookieValue(jar, "clientInfo")
	if raw == "" {
		return ""
	}
	padding := len(raw) % 4
	if padding != 0 {
		raw += strings.Repeat("=", 4-padding)
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return ""
	}

	var payload map[string]any
	if err = json.Unmarshal(decoded, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(anyToString(payload["sid"]))
}

// ensureCASTGCCookie writes the campus CASTGC cookie from the current TGT.
func (c *CampusClient) ensureCASTGCCookie(jar *cookiejar.Jar, tgt string) {
	if strings.TrimSpace(tgt) == "" {
		return
	}
	jar.SetCookies(c.baseURL(), []*http.Cookie{{
		Name:   "CASTGC",
		Value:  tgt,
		Path:   "/",
		Domain: c.baseURL().Hostname(),
		Secure: true,
	}})
}

// getCookieValue returns the value of the named cookie.
func (c *CampusClient) getCookieValue(jar *cookiejar.Jar, name string) string {
	for _, cookie := range jar.Cookies(c.baseURL()) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// newHTTPClient 鍒涘缓 HTTP 瀹㈡埛绔紝骞舵寜闇€鎺у埗鏄惁璺熼殢閲嶅畾鍚戙€
func (c *CampusClient) newHTTPClient(existingJar *cookiejar.Jar, followRedirect bool) (*http.Client, *cookiejar.Jar, error) {
	jar := existingJar
	var err error
	if jar == nil {
		jar, err = cookiejar.New(nil)
		if err != nil {
			return nil, nil, &CampusError{
				Code:    schoolpb.SchoolCode_SCHOOL_CODE_INTERNAL_ERROR,
				Message: fmt.Sprintf("鍒涘缓 cookie jar 澶辫触: %v", err),
			}
		}
	}

	client := &http.Client{
		Timeout: c.config.Timeout,
		Jar:     jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.config.InsecureSkipVerify}, //nolint:gosec
		},
	}
	if !followRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, jar, nil
}

// doGet sends a GET request and returns the decoded response body.
func (c *CampusClient) doGet(ctx context.Context, client *http.Client, rawURL string, headers map[string]string) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, "", err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("request campus system failed: %v", err),
		}
	}
	defer resp.Body.Close()

	body, err := readMaybeCompressedBody(resp)
	if err != nil {
		return nil, 0, "", err
	}
	return body, resp.StatusCode, resp.Request.URL.String(), nil
}

// doPostForm sends a form POST request and returns the decoded response body.
func (c *CampusClient) doPostForm(ctx context.Context, client *http.Client, rawURL string, form url.Values, headers map[string]string) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, "", err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("submit campus request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	body, err := readMaybeCompressedBody(resp)
	if err != nil {
		return nil, 0, "", err
	}
	return body, resp.StatusCode, resp.Request.URL.String(), nil
}

// htmlHeaders builds headers used when fetching campus HTML pages.
func (c *CampusClient) htmlHeaders(referer string) map[string]string {
	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":      c.config.UserAgent,
	}
	if referer != "" {
		headers["Referer"] = referer
	}
	return headers
}

// ajaxHeaders builds headers used when calling campus Ajax endpoints.
func (c *CampusClient) ajaxHeaders(jar *cookiejar.Jar, referer string) map[string]string {
	headers := map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Content-Type":     "application/x-www-form-urlencoded;charset=UTF-8",
		"Origin":           c.config.VPNBaseURL,
		"Referer":          referer,
		"User-Agent":       c.config.UserAgent,
		"X-Requested-With": "XMLHttpRequest",
		"loginToken":       c.config.LoginToken,
		"loginUserToken":   c.config.LoginUserToken,
	}
	headers["Cookie"] = joinCookies(jar.Cookies(c.baseURL()))
	return headers
}

func (c *CampusClient) casLoginURL() string {
	return strings.TrimRight(c.config.CASRoot, "/") + "/login?service=" + url.QueryEscape(c.config.TargetServiceURL)
}

func (c *CampusClient) captchaURL() string {
	return strings.TrimRight(c.config.CASRoot, "/") + "/kaptcha?_t=" + fmt.Sprintf("%d", time.Now().UnixMilli()) + "&uid=&enlink-vpn"
}

func (c *CampusClient) ticketsURL() string {
	return strings.TrimRight(c.config.CASRoot, "/") + "/v1/tickets?enlink-vpn"
}

func (c *CampusClient) ticketExchangeURL(tgt string) string {
	return strings.TrimRight(c.config.CASRoot, "/") + "/v1/tickets/" + url.PathEscape(tgt) + "?enlink-vpn"
}

func (c *CampusClient) initMenuURL() string {
	return strings.TrimRight(c.config.JWBaseURL, "/") + "/xtgl/index_initMenu.html?jsdm=&_t=" + fmt.Sprintf("%d", time.Now().UnixMilli())
}

func (c *CampusClient) scheduleIndexURL(studentNo string) string {
	if strings.TrimSpace(c.config.ScheduleIndexURL) != "" {
		return c.config.ScheduleIndexURL
	}
	return strings.TrimRight(c.config.JWBaseURL, "/") + "/kbcx/xskbcx_cxXskbcxIndex.html?gnmkdm=N2151&layout=default&su=" + url.QueryEscape(studentNo)
}

func (c *CampusClient) scheduleDataURL(studentNo string) string {
	base := strings.TrimSpace(c.config.ScheduleDataURL)
	if base == "" {
		base = strings.TrimRight(c.config.JWBaseURL, "/") + "/kbcx/xskbcx_cxXsgrkb.html?gnmkdm=N2151"
	}
	if strings.Contains(base, "su=") {
		return base
	}
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return base + separator + "su=" + url.QueryEscape(studentNo)
}

func (c *CampusClient) baseURL() *url.URL {
	parsed, _ := url.Parse(c.config.VPNBaseURL)
	return parsed
}

func (c *CampusClient) isPersonalInfoHTML(htmlText string) bool {
	return strings.Contains(htmlText, "curXh_id") || strings.Contains(htmlText, "xh_id") || strings.Contains(htmlText, "xsgrxxwh")
}

// encryptPassword applies the legacy RSA routine expected by the campus login page.
func (c *CampusClient) encryptPassword(password string) (string, error) {
	modulus := new(big.Int)
	if _, ok := modulus.SetString(c.config.RSAModulusHex, 16); !ok {
		return "", fmt.Errorf("invalid modulus")
	}

	exponent := new(big.Int)
	if _, ok := exponent.SetString(c.config.RSAExponentHex, 16); !ok {
		return "", fmt.Errorf("invalid exponent")
	}

	hexText := strings.TrimLeft(c.config.RSAModulusHex, "0")
	if hexText == "" {
		hexText = "0"
	}

	var digits []int64
	for end := len(hexText); end > 0; end -= 4 {
		start := end - 4
		if start < 0 {
			start = 0
		}
		partValue := new(big.Int)
		if _, ok := partValue.SetString(hexText[start:end], 16); !ok {
			return "", fmt.Errorf("invalid modulus chunk")
		}
		digits = append(digits, partValue.Int64())
	}
	for len(digits) > 1 && digits[len(digits)-1] == 0 {
		digits = digits[:len(digits)-1]
	}

	highestIndex := len(digits) - 1
	chunkSize := 2 * highestIndex
	if chunkSize <= 0 {
		return "", fmt.Errorf("invalid modulus size")
	}

	charCodes := make([]int, 0, len(password))
	for _, char := range password {
		charCodes = append(charCodes, int(char))
	}
	for len(charCodes)%chunkSize != 0 {
		charCodes = append(charCodes, 0)
	}

	encryptedBlocks := make([]string, 0, len(charCodes)/chunkSize)
	for offset := 0; offset < len(charCodes); offset += chunkSize {
		block := charCodes[offset : offset+chunkSize]
		value := new(big.Int)
		for pairIndex := 0; pairIndex < len(block); pairIndex += 2 {
			low := block[pairIndex]
			high := 0
			if pairIndex+1 < len(block) {
				high = block[pairIndex+1]
			}
			digit := int64(low + (high << 8))
			value.Add(value, new(big.Int).Lsh(big.NewInt(digit), uint(16*(pairIndex/2))))
		}
		encrypted := new(big.Int).Exp(value, exponent, modulus)
		encryptedBlocks = append(encryptedBlocks, encrypted.Text(16))
	}
	return strings.Join(encryptedBlocks, " "), nil
}

// parseLoginResponse 解析学校登录接口返回的 JSON。
func parseLoginResponse(body []byte) (*AuthorizeResult, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("解析登录响应失败: %v", err),
		}
	}

	if dataMap, ok := payload["data"].(map[string]any); ok {
		if code := strings.TrimSpace(anyToString(dataMap["code"])); code != "" {
			return nil, &CampusError{
				Code:    schoolpb.SchoolCode_SCHOOL_CODE_INVALID_CREDENTIAL,
				Message: code,
			}
		}
	}

	result := &AuthorizeResult{
		TGT:         strings.TrimSpace(anyToString(payload["tgt"])),
		Ticket:      strings.TrimSpace(anyToString(payload["ticket"])),
		StudentName: strings.TrimSpace(anyToString(payload["name"])),
	}

	if result.TGT == "" {
		switch data := payload["data"].(type) {
		case string:
			result.TGT = strings.TrimSpace(data)
		case map[string]any:
			result.TGT = strings.TrimSpace(anyToString(data["tgt"]))
			result.Ticket = strings.TrimSpace(anyToString(data["ticket"]))
			if result.StudentName == "" {
				result.StudentName = strings.TrimSpace(anyToString(data["name"]))
			}
		}
	}

	if result.TGT == "" && result.Ticket == "" {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_INVALID_CREDENTIAL,
			Message: "school did not return a valid login ticket",
		}
	}

	return result, nil
}

// parseProfileFields extracts visible profile fields from the personal-info page.
func parseStudentProfile(htmlText string) *types.StudentProfilePayload {
	hidden := parseHiddenInputs(htmlText)
	fields := parseProfileFields(htmlText)

	studentID := strings.TrimSpace(hidden["xh_id"])
	if studentID == "" {
		studentID = strings.TrimSpace(hidden["curXh_id"])
	}
	if studentID == "" {
		studentID = strings.TrimSpace(fields["学号"])
	}

	studentIDCode := strings.TrimSpace(hidden["xh_id_code"])
	if studentIDCode == "" {
		studentIDCode = strings.TrimSpace(hidden["curXh_id_code"])
	}

	return &types.StudentProfilePayload{
		FetchedAt:     time.Now().Format("2006-01-02T15:04:05"),
		StudentID:     studentID,
		StudentIDCode: studentIDCode,
		HiddenFields:  hidden,
		Fields:        fields,
	}
}

func parseHiddenInputs(htmlText string) map[string]string {
	result := make(map[string]string)
	re := regexp.MustCompile(`(?is)<input[^>]+type=["']hidden["'][^>]*name=["']([^"']+)["'][^>]*value=["']([^"']*)["'][^>]*>`)
	for _, match := range re.FindAllStringSubmatch(htmlText, -1) {
		name := strings.TrimSpace(html.UnescapeString(match[1]))
		value := strings.TrimSpace(html.UnescapeString(match[2]))
		if name == "" {
			continue
		}
		result[name] = value
	}
	return result
}

func parseProfileFields(htmlText string) map[string]string {
	result := make(map[string]string)
	re := regexp.MustCompile(`(?is)<div class="form-group"[^>]*>.*?<label[^>]*>(.*?)</label>.*?<p[^>]*class="form-control-static"[^>]*>(.*?)</p>`)
	for _, match := range re.FindAllStringSubmatch(htmlText, -1) {
		label := cleanHTMLText(match[1])
		value := cleanHTMLText(match[2])
		if label == "" || value == "" {
			continue
		}
		if _, exists := result[label]; !exists {
			result[label] = value
		}
	}
	return result
}

// cleanHTMLText strips tags and normalizes text extracted from HTML pages.
func cleanHTMLText(text string) string {
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe := regexp.MustCompile(`\s+`)
	text = tagRe.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = spaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(strings.Trim(text, `?:;,.!()[]{}<>"'`))
}

// extractJSRedirectURL extracts redirect targets from inline JavaScript.
func extractJSRedirectURL(htmlText, baseURL string) string {
	patterns := []string{
		`(?is)(?:window\.|top\.|self\.|document\.)?location(?:\.href)?\s*=\s*["']([^"']+)["']`,
		`(?is)(?:window\.|top\.|self\.|document\.)?location\.replace\(\s*["']([^"']+)["']\s*\)`,
		`(?is)(?:window\.|top\.|self\.|document\.)?location\.assign\(\s*["']([^"']+)["']\s*\)`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(htmlText)
		if len(match) != 2 {
			continue
		}
		baseParsed, err := url.Parse(baseURL)
		if err != nil {
			return ""
		}
		target, err := url.Parse(html.UnescapeString(match[1]))
		if err != nil {
			return ""
		}
		return baseParsed.ResolveReference(target).String()
	}
	return ""
}

// extractMetaRefreshURL extracts redirect targets from meta refresh tags.
func extractMetaRefreshURL(htmlText, baseURL string) string {
	re := regexp.MustCompile(`(?is)<meta[^>]+http-equiv=["']refresh["'][^>]+content=["'][^"']*url=([^"'>]+)`)
	match := re.FindStringSubmatch(htmlText)
	if len(match) != 2 {
		return ""
	}
	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	target, err := url.Parse(strings.TrimSpace(html.UnescapeString(match[1])))
	if err != nil {
		return ""
	}
	return baseParsed.ResolveReference(target).String()
}

// parseAutoSubmitForm extracts an auto-submit HTML form.
func parseAutoSubmitForm(htmlText, baseURL string) (string, string, map[string]string) {
	formRe := regexp.MustCompile(`(?is)<form\b([^>]*)>(.*?)</form>`)
	formMatch := formRe.FindStringSubmatch(htmlText)
	if len(formMatch) != 3 {
		return "", "", nil
	}

	attrs := formMatch[1]
	body := formMatch[2]

	action := baseURL
	actionRe := regexp.MustCompile(`(?is)action=["']([^"']*)["']`)
	if match := actionRe.FindStringSubmatch(attrs); len(match) == 2 {
		baseParsed, err := url.Parse(baseURL)
		if err == nil {
			target, err := url.Parse(html.UnescapeString(match[1]))
			if err == nil {
				action = baseParsed.ResolveReference(target).String()
			}
		}
	}

	method := http.MethodGet
	methodRe := regexp.MustCompile(`(?is)method=["']([^"']*)["']`)
	if match := methodRe.FindStringSubmatch(attrs); len(match) == 2 {
		method = strings.ToUpper(strings.TrimSpace(match[1]))
	}

	fields := make(map[string]string)
	inputRe := regexp.MustCompile(`(?is)<input[^>]+name=["']([^"']+)["'][^>]*value=["']([^"']*)["']`)
	for _, match := range inputRe.FindAllStringSubmatch(body, -1) {
		fields[html.UnescapeString(match[1])] = html.UnescapeString(match[2])
	}

	autoSubmit := strings.Contains(htmlText, "submit()") ||
		strings.Contains(htmlText, "document.forms[0]") ||
		strings.Contains(strings.ToLower(htmlText), "auto-submit")
	if !autoSubmit && len(fields) == 0 {
		return "", "", nil
	}

	return action, method, fields
}

// readMaybeCompressedBody 鑷姩瑙ｅ帇 gzip 鍝嶅簲浣撱€
func readMaybeCompressedBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	contentEncoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	if strings.Contains(contentEncoding, "gzip") || bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err == nil {
			defer reader.Close()
			decompressed, readErr := io.ReadAll(reader)
			if readErr == nil {
				return decompressed, nil
			}
		}
	}
	return body, nil
}

// exportCookies serializes cookies for caching.
func exportCookies(cookies []*http.Cookie) []types.CookieState {
	result := make([]types.CookieState, 0, len(cookies))
	for _, cookie := range cookies {
		result = append(result, types.CookieState{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
		})
	}
	return result
}

// restoreCookies rebuilds cookies from cached values.
func restoreCookies(cookies []types.CookieState) []*http.Cookie {
	result := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		result = append(result, &http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
		})
	}
	return result
}

// joinCookies joins cookies into a request header string.
func joinCookies(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
}

// anyToString converts an arbitrary JSON value into a string.
func anyToString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// randomID builds a simple prefixed identifier.
func randomID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// GetStudentSchedule 使用学校侧 TGT 进入教务系统并抓取课表。
func (c *CampusClient) GetStudentSchedule(ctx context.Context, session types.SchoolSession, term string) ([]types.StudentScheduleItem, []types.ScheduleSectionTime, int32, error) {
	startedAt := time.Now()
	c.appendScheduleDebug("start user=%d studentNo=%s term=%s", session.UserID, session.StudentNo, term)

	client, jar, err := c.newHTTPClient(nil, true)
	if err != nil {
		c.appendScheduleDebug("newHTTPClient error=%v", err)
		return nil, nil, 0, err
	}
	c.ensureCASTGCCookie(jar, session.TGT)
	c.appendScheduleDebug("castgc prepared")

	if err := c.bootstrapSession(ctx, client); err != nil {
		c.appendScheduleDebug("bootstrapSession error=%v", err)
		return nil, nil, 0, err
	}
	c.appendScheduleDebug("bootstrapSession ok elapsed=%s", time.Since(startedAt))

	body, status, finalURL, err := c.requestPersonalInfoPage(ctx, client, c.casLoginURL())
	if err != nil {
		c.appendScheduleDebug("requestPersonalInfoPage error=%v", err)
		return nil, nil, 0, err
	}
	c.appendScheduleDebug("requestPersonalInfoPage status=%d finalURL=%s", status, finalURL)

	if status != http.StatusOK || !c.isPersonalInfoHTML(string(body)) {
		c.appendScheduleDebug("personalInfo direct access miss, fallback to sso")

		callbackService := strings.TrimRight(c.config.VPNBaseURL, "/") + "/enlink/api/client/callback/cas"
		callbackTicket, err := c.exchangeServiceTicket(ctx, client, jar, session.TGT, callbackService)
		if err != nil {
			c.appendScheduleDebug("exchange callback ticket error=%v", err)
			return nil, nil, 0, err
		}
		if strings.TrimSpace(callbackTicket) == "" {
			return nil, nil, 0, &CampusError{
				Code:    schoolpb.SchoolCode_SCHOOL_CODE_SESSION_EXPIRED,
				Message: "学校 TGT 已失效，无法换取 callback ticket",
			}
		}

		noRedirectClient, _, err := c.newHTTPClient(jar, false)
		if err != nil {
			c.appendScheduleDebug("new noRedirectClient error=%v", err)
			return nil, nil, 0, err
		}
		callbackURL := callbackService + "?ticket=" + url.QueryEscape(callbackTicket)
		if _, _, _, err = c.doGet(ctx, noRedirectClient, callbackURL, c.htmlHeaders(finalURL)); err != nil {
			c.appendScheduleDebug("callback doGet error=%v", err)
			return nil, nil, 0, err
		}

		entoken := c.decodeClientSID(jar)
		if entoken == "" {
			return nil, nil, 0, &CampusError{
				Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
				Message: "未从 clientInfo cookie 解析到 Enlink sid",
			}
		}

		jwTicket, err := c.exchangeServiceTicket(ctx, client, jar, session.TGT, c.config.TargetServiceURL)
		if err != nil {
			c.appendScheduleDebug("exchange jw ticket error=%v", err)
			return nil, nil, 0, err
		}
		if strings.TrimSpace(jwTicket) == "" {
			return nil, nil, 0, &CampusError{
				Code:    schoolpb.SchoolCode_SCHOOL_CODE_SESSION_EXPIRED,
				Message: "学校 TGT 已失效，无法换取教务系统 ticket",
			}
		}

		lyiotURL := strings.TrimRight(c.config.JWBaseURL, "/") + "/sso/lyiotlogin?ticket=" + url.QueryEscape(jwTicket)
		authorizedURL := strings.TrimRight(c.config.VPNBaseURL, "/") +
			"/engateway/api/sso/authorized?redirect_url=" + url.QueryEscape(lyiotURL) +
			"&entoken=" + url.QueryEscape(entoken)
		body, _, finalURL, err = c.doGet(ctx, client, authorizedURL, c.htmlHeaders(callbackURL))
		if err != nil {
			c.appendScheduleDebug("authorized doGet error=%v", err)
			return nil, nil, 0, err
		}

		_, finalURL, err = c.walkHTMLSSO(ctx, client, string(body), finalURL, authorizedURL)
		if err != nil {
			c.appendScheduleDebug("walkHTMLSSO error=%v", err)
			return nil, nil, 0, err
		}
		c.appendScheduleDebug("walkHTMLSSO ok finalURL=%s elapsed=%s", finalURL, time.Since(startedAt))
	}

	menuURL := c.initMenuURL()
	if _, _, initializedMenuURL, err := c.doGet(ctx, client, menuURL, c.htmlHeaders(finalURL)); err == nil && strings.TrimSpace(initializedMenuURL) != "" {
		menuURL = initializedMenuURL
	}
	c.appendScheduleDebug("initMenu final menuURL=%s", menuURL)

	body, status, finalURL, err = c.doGet(ctx, client, c.scheduleIndexURL(session.StudentNo), c.htmlHeaders(menuURL))
	if err != nil {
		c.appendScheduleDebug("schedule index doGet error=%v", err)
		return nil, nil, 0, err
	}
	if status != http.StatusOK {
		return nil, nil, 0, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("获取课表首页失败: http %d", status),
		}
	}

	indexHTML := string(body)
	xnm, xqm := parseScheduleTerm(term, indexHTML)
	currentWeek := parseScheduleCurrentWeek(indexHTML)
	sectionTimes := ensureScheduleSectionTimes(parseScheduleSectionTimes(indexHTML))
	c.appendScheduleDebug("schedule index ok xnm=%s xqm=%s currentWeek=%d", xnm, xqm, currentWeek)

	form := url.Values{}
	form.Set("xnm", xnm)
	form.Set("xqm", xqm)
	form.Set("kzlx", "ck")
	form.Set("xsdm", "")

	body, status, _, err = c.doPostForm(ctx, client, c.scheduleDataURL(session.StudentNo), form, c.ajaxHeaders(jar, finalURL))
	if err != nil {
		c.appendScheduleDebug("schedule data doPost error=%v", err)
		return nil, nil, currentWeek, err
	}
	if status != http.StatusOK {
		return nil, nil, currentWeek, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("获取课表数据失败: http %d", status),
		}
	}

	items, err := parseScheduleJSON(body)
	if err == nil && len(items) > 0 {
		sectionTimes = ensureScheduleSectionTimes(mergeScheduleSectionTimes(sectionTimes, extractSectionTimesFromItems(items)))
		c.appendScheduleDebug("parseScheduleJSON ok count=%d totalElapsed=%s", len(items), time.Since(startedAt))
		return items, sectionTimes, currentWeek, nil
	}

	htmlItems := parseScheduleFromHTML(string(body))
	if len(htmlItems) > 0 {
		sectionTimes = ensureScheduleSectionTimes(mergeScheduleSectionTimes(sectionTimes, extractSectionTimesFromItems(htmlItems)))
		c.appendScheduleDebug("parseScheduleFromHTML ok count=%d totalElapsed=%s", len(htmlItems), time.Since(startedAt))
		return htmlItems, sectionTimes, currentWeek, nil
	}

	if err != nil {
		c.appendScheduleDebug("parse schedule final error=%v", err)
		return nil, nil, currentWeek, err
	}

	return nil, nil, currentWeek, &CampusError{
		Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
		Message: "schedule items empty after parsing",
	}
}

// parseScheduleTerm resolves the academic term, preferring the explicit caller input when provided.
func parseScheduleTerm(term string, htmlText string) (string, string) {
	parts := splitTerm(term)
	if len(parts) >= 2 {
		xnm := strings.TrimSpace(parts[0])
		xqm := normalizeTermCode(parts[1])
		if xnm != "" && xqm != "" {
			return xnm, xqm
		}
	}

	hidden := parseHiddenInputs(htmlText)
	xnm := strings.TrimSpace(hidden["xnm"])
	xqm := normalizeTermCode(hidden["xqm"])
	if xnm != "" && xqm != "" {
		return xnm, xqm
	}

	xnm = firstMatchedGroup(htmlText, `(?is)id=["']xnm["'][^>]*>.*?<option[^>]*selected[^>]*value=["']([^"']+)["']`)
	xqm = normalizeTermCode(firstMatchedGroup(htmlText, `(?is)id=["']xqm["'][^>]*>.*?<option[^>]*selected[^>]*value=["']([^"']+)["']`))
	if xnm != "" && xqm != "" {
		return xnm, xqm
	}

	now := time.Now()
	if now.Month() >= 9 {
		return strconv.Itoa(now.Year()), "3"
	}
	return strconv.Itoa(now.Year() - 1), "12"
}

// parseScheduleCurrentWeek 从课表首页提取学校当前周次。
// 这里优先解析隐藏字段，其次匹配页面文案和脚本变量，避免前端自行猜测周次。
func parseScheduleCurrentWeek(htmlText string) int32 {
	hidden := parseHiddenInputs(htmlText)
	for _, key := range []string{"zc", "dqzc", "currentWeek", "week", "xqzc"} {
		if week := parsePositiveWeek(hidden[key]); week > 0 {
			return week
		}
	}

	patterns := []string{
		`第\s*(\d{1,2})\s*周`,
		`当前周[^0-9]{0,8}(\d{1,2})`,
		`(?:currentWeek|week|dqzc|xqzc)\s*[:=]\s*["']?(\d{1,2})`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(htmlText); len(match) >= 2 {
			if week := parsePositiveWeek(match[1]); week > 0 {
				return week
			}
		}
	}

	return 0
}

func parsePositiveWeek(raw string) int32 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	match := regexp.MustCompile(`\d{1,2}`).FindString(value)
	if match == "" {
		return 0
	}
	week, err := strconv.Atoi(match)
	if err != nil || week <= 0 || week > 30 {
		return 0
	}
	return int32(week)
}

// parseScheduleJSON parses the course schedule JSON payload returned by the campus system.
func parseScheduleJSON(body []byte) ([]types.StudentScheduleItem, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: fmt.Sprintf("parse schedule JSON failed: %v", err),
		}
	}

	list := findScheduleList(payload)
	if len(list) == 0 {
		return nil, &CampusError{
			Code:    schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR,
			Message: "schedule JSON missing kbList",
		}
	}

	items := make([]types.StudentScheduleItem, 0, len(list))
	for _, raw := range list {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		startSection := firstNonEmptyString(row, "ksjc", "jcs")
		endSection := firstNonEmptyString(row, "jsjc", "jcs")
		if strings.Contains(startSection, "-") {
			parts := strings.SplitN(startSection, "-", 2)
			startSection = strings.TrimSpace(parts[0])
			if strings.TrimSpace(endSection) == "" || endSection == firstNonEmptyString(row, "jcs") {
				endSection = strings.TrimSpace(parts[1])
			}
		}
		startTime := firstNonEmptyString(row, "kssj", "sksj", "start_time")
		endTime := firstNonEmptyString(row, "jssj", "xksj", "xssj", "end_time")
		if startTime == "" || endTime == "" {
			rangeStart, rangeEnd := splitClockRange(firstNonEmptyString(row, "jcsj", "sjjd", "time_range"))
			if startTime == "" {
				startTime = rangeStart
			}
			if endTime == "" {
				endTime = rangeEnd
			}
		}

		items = append(items, types.StudentScheduleItem{
			CourseName:   firstNonEmptyString(row, "kcmc", "course_name"),
			Teacher:      firstNonEmptyString(row, "xm", "jsxm", "teacher"),
			Classroom:    firstNonEmptyString(row, "cdmc", "jxdd", "classroom"),
			Weekday:      normalizeWeekday(firstNonEmptyString(row, "xqjmc", "xqj")),
			StartSection: startSection,
			EndSection:   endSection,
			Weeks:        firstNonEmptyString(row, "zcd", "week", "weeks"),
			StartTime:    normalizeClockText(startTime),
			EndTime:      normalizeClockText(endTime),
		})
	}

	return compactScheduleItems(items), nil
}

// parseScheduleFromHTML is the fallback parser for schedule blocks embedded in HTML.
func parseScheduleFromHTML(htmlText string) []types.StudentScheduleItem {
	re := regexp.MustCompile(`(?is)<div[^>]*class=["'][^"']*kbcontent[^"']*["'][^>]*>(.*?)</div>`)
	matches := re.FindAllStringSubmatch(htmlText, -1)
	items := make([]types.StudentScheduleItem, 0, len(matches))
	for _, match := range matches {
		text := cleanHTMLText(match[1])
		if text == "" {
			continue
		}

		lines := splitScheduleLines(text)
		if len(lines) == 0 {
			continue
		}

		items = append(items, types.StudentScheduleItem{
			CourseName: lines[0],
			Teacher:    lineAt(lines, 1),
			Classroom:  lineAt(lines, 2),
			Weeks:      lineAt(lines, 3),
		})
	}

	return compactScheduleItems(items)
}

func findScheduleList(payload map[string]any) []any {
	for _, key := range []string{"kbList", "data", "items"} {
		if list, ok := payload[key].([]any); ok {
			return list
		}
		if nested, ok := payload[key].(map[string]any); ok {
			if list := findScheduleList(nested); len(list) > 0 {
				return list
			}
		}
	}
	return nil
}

func firstNonEmptyString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(anyToString(row[key])); value != "" {
			return value
		}
	}
	return ""
}

func compactScheduleItems(items []types.StudentScheduleItem) []types.StudentScheduleItem {
	result := make([]types.StudentScheduleItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.CourseName) == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

// parseScheduleSectionTimes tries to parse real section times from the schedule HTML.
func parseScheduleSectionTimes(htmlText string) []types.ScheduleSectionTime {
	patterns := []string{
		`(?s)(\d{1,2})[^\d:]{0,16}(\d{1,2}:\d{2})\s*[-~至到]\s*(\d{1,2}:\d{2})`,
		`(?s)data-section=["']?(\d{1,2})["']?[^>]*data-start=["']?(\d{1,2}:\d{2})["']?[^>]*data-end=["']?(\d{1,2}:\d{2})["']?`,
		`(?s)(\d{1,2})[^\d:]{0,24}(\d{1,2}:\d{2})[^\d:]{0,24}(\d{1,2}:\d{2})`,
	}

	seen := make(map[string]types.ScheduleSectionTime)
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(htmlText, -1)
		for _, match := range matches {
			if len(match) < 4 {
				continue
			}
			section := strings.TrimSpace(match[1])
			startTime := normalizeClockText(match[2])
			endTime := normalizeClockText(match[3])
			if section == "" || startTime == "" || endTime == "" {
				continue
			}
			seen[section] = types.ScheduleSectionTime{
				Section:   section,
				StartTime: startTime,
				EndTime:   endTime,
			}
		}
	}

	return sortScheduleSectionTimes(mapValuesToSectionTimes(seen))
}

// extractSectionTimesFromItems fills missing section times from parsed schedule items.
func extractSectionTimesFromItems(items []types.StudentScheduleItem) []types.ScheduleSectionTime {
	seen := make(map[string]types.ScheduleSectionTime)
	for _, item := range items {
		section := strings.TrimSpace(item.StartSection)
		startTime := normalizeClockText(item.StartTime)
		endTime := normalizeClockText(item.EndTime)
		if section == "" || startTime == "" || endTime == "" {
			continue
		}
		seen[section] = types.ScheduleSectionTime{
			Section:   section,
			StartTime: startTime,
			EndTime:   endTime,
		}
	}
	return sortScheduleSectionTimes(mapValuesToSectionTimes(seen))
}

// mergeScheduleSectionTimes merges homepage section times with item-level section times.
func mergeScheduleSectionTimes(primary []types.ScheduleSectionTime, fallback []types.ScheduleSectionTime) []types.ScheduleSectionTime {
	merged := make(map[string]types.ScheduleSectionTime)
	for _, item := range fallback {
		if strings.TrimSpace(item.Section) == "" {
			continue
		}
		merged[item.Section] = item
	}
	for _, item := range primary {
		if strings.TrimSpace(item.Section) == "" {
			continue
		}
		merged[item.Section] = item
	}
	return sortScheduleSectionTimes(mapValuesToSectionTimes(merged))
}

// ensureScheduleSectionTimes fills in a fallback timetable when the campus page does not expose section times.
func ensureScheduleSectionTimes(items []types.ScheduleSectionTime) []types.ScheduleSectionTime {
	if len(items) > 0 {
		return items
	}

	return []types.ScheduleSectionTime{
		{Section: "1", StartTime: "08:00", EndTime: "08:50"},
		{Section: "2", StartTime: "09:00", EndTime: "09:50"},
		{Section: "3", StartTime: "10:10", EndTime: "11:00"},
		{Section: "4", StartTime: "11:10", EndTime: "12:00"},
		{Section: "5", StartTime: "13:30", EndTime: "14:20"},
		{Section: "6", StartTime: "14:30", EndTime: "15:20"},
		{Section: "7", StartTime: "15:40", EndTime: "16:30"},
		{Section: "8", StartTime: "16:40", EndTime: "17:30"},
		{Section: "9", StartTime: "18:30", EndTime: "19:20"},
		{Section: "10", StartTime: "19:30", EndTime: "20:20"},
		{Section: "11", StartTime: "20:30", EndTime: "21:20"},
		{Section: "12", StartTime: "21:30", EndTime: "22:20"},
	}
}

func mapValuesToSectionTimes(values map[string]types.ScheduleSectionTime) []types.ScheduleSectionTime {
	result := make([]types.ScheduleSectionTime, 0, len(values))
	for _, item := range values {
		result = append(result, item)
	}
	return result
}

func sortScheduleSectionTimes(items []types.ScheduleSectionTime) []types.ScheduleSectionTime {
	sort.Slice(items, func(i, j int) bool {
		left, _ := strconv.Atoi(strings.TrimSpace(items[i].Section))
		right, _ := strconv.Atoi(strings.TrimSpace(items[j].Section))
		return left < right
	})
	return items
}

func normalizeClockText(raw string) string {
	match := regexp.MustCompile(`\d{1,2}:\d{2}`).FindString(strings.TrimSpace(raw))
	return strings.TrimSpace(match)
}

func splitClockRange(raw string) (string, string) {
	matches := regexp.MustCompile(`\d{1,2}:\d{2}`).FindAllString(strings.TrimSpace(raw), -1)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[0]), strings.TrimSpace(matches[1])
	}
	return "", ""
}

func splitTerm(term string) []string {
	replacer := strings.NewReplacer("??", "-", "??", "", "/", "-", "_", "-", " ", "")
	normalized := replacer.Replace(strings.TrimSpace(term))
	if normalized == "" {
		return nil
	}
	return strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '-' || r == ','
	})
}

func normalizeTermCode(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "1", "3":
		return "3"
	case "2", "12":
		return "12"
	default:
		return raw
	}
}

func normalizeWeekday(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "1":
		return "周一"
	case "2":
		return "周二"
	case "3":
		return "周三"
	case "4":
		return "周四"
	case "5":
		return "周五"
	case "6":
		return "周六"
	case "7":
		return "周日"
	default:
		return raw
	}
}

func firstMatchedGroup(text string, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

func splitScheduleLines(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func lineAt(lines []string, index int) string {
	if index < 0 || index >= len(lines) {
		return ""
	}
	return lines[index]
}

func (c *CampusClient) appendScheduleDebug(format string, args ...any) {
	logDir := filepath.Join("..", "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logPath := filepath.Join(logDir, "school-schedule-debug.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()

	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	_, _ = file.WriteString(line)
}


