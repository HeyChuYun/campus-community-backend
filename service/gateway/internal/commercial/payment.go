package commercial

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"hfjj/backend/gateway/internal/config"

	"github.com/go-pay/gopay"
	alipayv3 "github.com/go-pay/gopay/alipay/v3"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"
)

type PaymentProvider interface {
	Channel() string
	BuildAppPay(ctx context.Context, order PaymentOrder) map[string]any
	ParseNotify(r *http.Request) (PaymentNotify, error)
	SuccessResponse() []byte
	FailureResponse(message string) []byte
}

type PaymentOrder struct {
	BizType     string
	OrderNo     string
	AmountCents int
	Subject     string
}

type PaymentNotify struct {
	OrderNo         string
	ProviderTradeNo string
	Status          string
	Success         bool
	Payload         any
}

type PaymentRegistry struct {
	providers map[string]PaymentProvider
}

func NewPaymentRegistry(payment config.PaymentConfig) *PaymentRegistry {
	return &PaymentRegistry{providers: map[string]PaymentProvider{
		PayChannelAlipay: newAlipayProvider(payment.Alipay),
		PayChannelWechat: newWechatProvider(payment.Wechat),
	}}
}

func (r *PaymentRegistry) Provider(channel string) (PaymentProvider, error) {
	if r == nil {
		return nil, ErrUnsupportedProvider
	}
	provider, ok := r.providers[strings.ToLower(strings.TrimSpace(channel))]
	if !ok {
		return nil, ErrUnsupportedProvider
	}
	return provider, nil
}

type alipayProvider struct {
	cfg     config.PaymentAlipayConfig
	client  *alipayv3.ClientV3
	initErr error
}

func newAlipayProvider(cfg config.PaymentAlipayConfig) PaymentProvider {
	provider := &alipayProvider{cfg: cfg}
	provider.init()
	return provider
}

func (p *alipayProvider) init() {
	// 保持延迟初始化语义：未配置时允许服务启动，下单时再返回明确提示。
	if strings.TrimSpace(p.cfg.AppID) == "" ||
		strings.TrimSpace(p.cfg.PrivateKey) == "" ||
		strings.TrimSpace(p.cfg.AlipayPublicKey) == "" {
		return
	}

	client, err := alipayv3.NewClientV3(
		strings.TrimSpace(p.cfg.AppID),
		strings.TrimSpace(p.cfg.PrivateKey),
		!p.cfg.Sandbox,
	)
	if err != nil {
		p.initErr = fmt.Errorf("初始化支付宝客户端失败: %w", err)
		return
	}
	p.client = client
}

func (p *alipayProvider) Channel() string { return PayChannelAlipay }

func (p *alipayProvider) BuildAppPay(ctx context.Context, order PaymentOrder) map[string]any {
	params := basePayParams(order, p.Channel())
	params["notify_url"] = p.cfg.NotifyURL
	params["return_url"] = p.cfg.ReturnURL
	params["sandbox"] = p.cfg.Sandbox

	if !p.isConfigured() {
		params["mode"] = "provider_unconfigured"
		params["message"] = "请先配置支付宝 AppID、应用私钥和支付宝公钥"
		return params
	}
	if p.initErr != nil {
		params["mode"] = "provider_error"
		params["message"] = p.initErr.Error()
		return params
	}

	// 这里改为走 gopay 生成 app 支付订单串，减少手写签名和参数拼装代码。
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", order.OrderNo).
		Set("total_amount", formatAmountYuan(order.AmountCents)).
		Set("subject", order.Subject).
		Set("product_code", "QUICK_MSECURITY_PAY").
		Set("timeout_express", "30m")
	if notifyURL := strings.TrimSpace(p.cfg.NotifyURL); notifyURL != "" {
		bm.Set("notify_url", notifyURL)
	}
	if returnURL := strings.TrimSpace(p.cfg.ReturnURL); returnURL != "" {
		bm.Set("return_url", returnURL)
	}

	orderString, err := p.client.TradeAppPay(ctx, bm)
	if err != nil {
		params["mode"] = "provider_error"
		params["message"] = err.Error()
		return params
	}

	params["mode"] = "app_pay"
	params["order_string"] = orderString
	return params
}

func (p *alipayProvider) ParseNotify(r *http.Request) (PaymentNotify, error) {
	payload, err := parseAlipayNotify(r)
	if err != nil {
		return PaymentNotify{}, err
	}
	if err = verifyAlipaySignature(payload, p.cfg.AlipayPublicKey); err != nil {
		return PaymentNotify{}, err
	}
	orderNo := strings.TrimSpace(payload["out_trade_no"])
	if orderNo == "" {
		orderNo = strings.TrimSpace(payload["order_no"])
	}
	status := strings.TrimSpace(payload["trade_status"])
	return PaymentNotify{
		OrderNo:         orderNo,
		ProviderTradeNo: strings.TrimSpace(payload["trade_no"]),
		Status:          status,
		Success:         status == "TRADE_SUCCESS" || status == "TRADE_FINISHED",
		Payload:         payload,
	}, nil
}

func (p *alipayProvider) SuccessResponse() []byte { return []byte("success") }

func (p *alipayProvider) FailureResponse(_ string) []byte { return []byte("fail") }

func (p *alipayProvider) isConfigured() bool {
	return strings.TrimSpace(p.cfg.AppID) != "" &&
		strings.TrimSpace(p.cfg.PrivateKey) != "" &&
		strings.TrimSpace(p.cfg.AlipayPublicKey) != ""
}

type wechatProvider struct {
	cfg         config.PaymentWechatConfig
	client      *wechatv3.ClientV3
	wxPublicKey *rsa.PublicKey
	initErr     error
}

func newWechatProvider(cfg config.PaymentWechatConfig) PaymentProvider {
	provider := &wechatProvider{cfg: cfg}
	provider.init()
	return provider
}

func (p *wechatProvider) init() {
	// 同样保留未配置可启动的策略，避免支付配置影响整个网关启动。
	if strings.TrimSpace(p.cfg.AppID) == "" ||
		strings.TrimSpace(p.cfg.MchID) == "" ||
		strings.TrimSpace(p.cfg.APIv3Key) == "" ||
		strings.TrimSpace(p.cfg.SerialNo) == "" ||
		strings.TrimSpace(p.cfg.PrivateKey) == "" ||
		strings.TrimSpace(p.cfg.PlatformPublicKey) == "" {
		return
	}

	client, err := wechatv3.NewClientV3(
		strings.TrimSpace(p.cfg.MchID),
		strings.TrimSpace(p.cfg.SerialNo),
		strings.TrimSpace(p.cfg.APIv3Key),
		strings.TrimSpace(p.cfg.PrivateKey),
	)
	if err != nil {
		p.initErr = fmt.Errorf("初始化微信支付客户端失败: %w", err)
		return
	}

	wxPublicKey, err := parseRSAPublicKey(p.cfg.PlatformPublicKey)
	if err != nil {
		p.initErr = fmt.Errorf("解析微信支付平台公钥失败: %w", err)
		return
	}

	p.client = client
	p.wxPublicKey = wxPublicKey
}

func (p *wechatProvider) Channel() string { return PayChannelWechat }

func (p *wechatProvider) BuildAppPay(ctx context.Context, order PaymentOrder) map[string]any {
	params := basePayParams(order, p.Channel())
	params["notify_url"] = p.cfg.NotifyURL
	params["sandbox"] = p.cfg.Sandbox

	if !p.isConfigured() {
		params["mode"] = "provider_unconfigured"
		params["message"] = "请先配置微信支付商户参数和密钥"
		return params
	}
	if p.initErr != nil {
		params["mode"] = "provider_error"
		params["message"] = p.initErr.Error()
		return params
	}

	// 下单与二次签名都交给 gopay，避免继续手写 v3 请求签名逻辑。
	bm := make(gopay.BodyMap)
	bm.Set("appid", strings.TrimSpace(p.cfg.AppID)).
		Set("description", order.Subject).
		Set("out_trade_no", order.OrderNo).
		Set("notify_url", strings.TrimSpace(p.cfg.NotifyURL)).
		SetBodyMap("amount", func(amount gopay.BodyMap) {
			amount.Set("total", order.AmountCents).
				Set("currency", "CNY")
		})

	prepayRsp, err := p.client.V3TransactionApp(ctx, bm)
	if err != nil {
		params["mode"] = "provider_error"
		params["message"] = err.Error()
		return params
	}
	if prepayRsp == nil || prepayRsp.Code != 0 || prepayRsp.Response == nil || strings.TrimSpace(prepayRsp.Response.PrepayId) == "" {
		params["mode"] = "provider_error"
		params["message"] = formatWechatPayError(prepayRsp)
		return params
	}

	paySign, err := p.client.PaySignOfApp(strings.TrimSpace(p.cfg.AppID), strings.TrimSpace(prepayRsp.Response.PrepayId))
	if err != nil {
		params["mode"] = "provider_error"
		params["message"] = err.Error()
		return params
	}

	params["mode"] = "app_pay"
	// 保持返回字段和旧实现一致，减少前端改动。
	params["app_id"] = paySign.Appid
	params["partner_id"] = paySign.Partnerid
	params["prepay_id"] = paySign.Prepayid
	params["package"] = paySign.Package
	params["nonce_str"] = paySign.Noncestr
	params["timestamp"] = paySign.Timestamp
	params["sign"] = paySign.Sign
	return params
}

func (p *wechatProvider) ParseNotify(r *http.Request) (PaymentNotify, error) {
	if p.wxPublicKey == nil {
		return PaymentNotify{}, fmt.Errorf("wechat platform public key is not configured")
	}

	notifyReq, err := wechatv3.V3ParseNotify(r)
	if err != nil {
		return PaymentNotify{}, err
	}
	if err = notifyReq.VerifySignByPK(p.wxPublicKey); err != nil {
		return PaymentNotify{}, err
	}

	// 支付回调体用 gopay 解密，避免继续维护 AES-GCM 细节。
	result, err := notifyReq.DecryptPayCipherText(strings.TrimSpace(p.cfg.APIv3Key))
	if err != nil {
		return PaymentNotify{}, err
	}
	status := strings.TrimSpace(result.TradeState)
	return PaymentNotify{
		OrderNo:         strings.TrimSpace(result.OutTradeNo),
		ProviderTradeNo: strings.TrimSpace(result.TransactionId),
		Status:          status,
		Success:         strings.EqualFold(status, "SUCCESS") || strings.EqualFold(status, "TRADE_SUCCESS"),
		Payload:         result,
	}, nil
}

func (p *wechatProvider) SuccessResponse() []byte {
	return []byte(`{"code":"SUCCESS","message":"成功"}`)
}

func (p *wechatProvider) FailureResponse(message string) []byte {
	if strings.TrimSpace(message) == "" {
		message = "失败"
	}
	payload, _ := json.Marshal(map[string]string{"code": "FAIL", "message": message})
	return payload
}

func (p *wechatProvider) isConfigured() bool {
	return strings.TrimSpace(p.cfg.AppID) != "" &&
		strings.TrimSpace(p.cfg.MchID) != "" &&
		strings.TrimSpace(p.cfg.APIv3Key) != "" &&
		strings.TrimSpace(p.cfg.SerialNo) != "" &&
		strings.TrimSpace(p.cfg.PrivateKey) != "" &&
		strings.TrimSpace(p.cfg.PlatformPublicKey) != ""
}

func basePayParams(order PaymentOrder, channel string) map[string]any {
	return map[string]any{
		"biz_type":     order.BizType,
		"channel":      channel,
		"order_no":     order.OrderNo,
		"amount_cents": order.AmountCents,
		"subject":      order.Subject,
	}
}

func formatAmountYuan(amountCents int) string {
	return fmt.Sprintf("%.2f", float64(amountCents)/100)
}

func formatWechatPayError(rsp *wechatv3.PrepayRsp) string {
	if rsp == nil {
		return "wechat prepay response is nil"
	}
	if strings.TrimSpace(rsp.Error) != "" {
		return rsp.Error
	}
	if strings.TrimSpace(rsp.ErrResponse.Message) != "" {
		if strings.TrimSpace(rsp.ErrResponse.Code) != "" {
			return rsp.ErrResponse.Code + ": " + rsp.ErrResponse.Message
		}
		return rsp.ErrResponse.Message
	}
	return "wechat prepay failed"
}

func parseAlipayNotify(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	payload := map[string]string{}
	for key, values := range r.PostForm {
		if len(values) > 0 {
			payload[key] = values[0]
		}
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty notify payload")
	}
	return payload, nil
}

func verifyAlipaySignature(payload map[string]string, publicKeyText string) error {
	publicKey, err := parseRSAPublicKey(publicKeyText)
	if err != nil {
		return fmt.Errorf("alipay public key invalid: %w", err)
	}
	signatureText := strings.TrimSpace(payload["sign"])
	if signatureText == "" {
		return fmt.Errorf("missing sign")
	}
	signature, err := base64.StdEncoding.DecodeString(signatureText)
	if err != nil {
		return fmt.Errorf("invalid sign")
	}
	signType := strings.ToUpper(strings.TrimSpace(payload["sign_type"]))
	if signType == "" {
		signType = "RSA2"
	}

	keys := make([]string, 0, len(payload))
	for key := range payload {
		if key == "sign" || key == "sign_type" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+payload[key])
	}
	message := []byte(strings.Join(parts, "&"))
	switch signType {
	case "RSA2":
		digest := sha256.Sum256(message)
		return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
	case "RSA":
		digest := sha1.Sum(message)
		return rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, digest[:], signature)
	default:
		return fmt.Errorf("unsupported sign_type: %s", signType)
	}
}

func parseRSAPublicKey(publicKeyText string) (*rsa.PublicKey, error) {
	publicKeyText = normalizePEM(publicKeyText, "PUBLIC KEY")
	if publicKeyText == "" {
		return nil, fmt.Errorf("public key is empty")
	}
	block, _ := pem.Decode([]byte(publicKeyText))
	if block == nil {
		return nil, fmt.Errorf("pem decode failed")
	}
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if publicKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return publicKey, nil
		}
		return nil, fmt.Errorf("certificate public key is not rsa")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if pkcs1, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
			return pkcs1, nil
		}
		return nil, err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not rsa")
	}
	return publicKey, nil
}

func normalizePEM(value string, label string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, `\n`, "\n")
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "BEGIN") {
		return "-----BEGIN " + label + "-----\n" + value + "\n-----END " + label + "-----"
	}
	return value
}
