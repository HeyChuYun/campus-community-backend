package wukong

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hfjj/backend/im-service/internal/config"

	"github.com/google/uuid"
)

type Client struct {
	config     config.WuKongConfig
	httpClient *http.Client
}

type RouteInfo struct {
	TCPAddr string
	WSAddr  string
	WSSAddr string
}

func NewClient(cfg config.WuKongConfig) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func FormatIMUID(userID int64) string {
	return fmt.Sprintf("u_%d", userID)
}

// ParseIMUID 把 WuKongIM 的 uid 反解成平台 user_id。
func ParseIMUID(imUID string) (int64, bool) {
	if !strings.HasPrefix(strings.TrimSpace(imUID), "u_") {
		return 0, false
	}

	var userID int64
	_, err := fmt.Sscanf(strings.TrimSpace(imUID), "u_%d", &userID)
	if err != nil || userID <= 0 {
		return 0, false
	}
	return userID, true
}

func (c *Client) IssueConnectToken(ctx context.Context, imUID string, deviceFlag, deviceLevel int32) (string, RouteInfo, error) {
	if strings.TrimSpace(c.config.BaseURL) == "" {
		return "", RouteInfo{}, errors.New("wukong base url is empty")
	}

	connectToken := uuid.NewString()

	// 第一步：把平台用户映射成 WuKongIM 用户，并下发当前连接 token。
	if _, err := c.post(ctx, c.tokenUpdatePath(), map[string]any{
		"uid":          imUID,
		"token":        connectToken,
		"device_flag":  deviceFlag,
		"device_level": deviceLevel,
	}); err != nil {
		return "", RouteInfo{}, err
	}

	// 第二步：向 WuKongIM 获取客户端应该连接到的 TCP / WS 路由地址。
	route, err := c.getRoute(ctx, imUID, connectToken, deviceFlag, deviceLevel)
	if err != nil {
		return "", RouteInfo{}, err
	}
	return connectToken, route, nil
}

func (c *Client) SendDirectMessage(ctx context.Context, fromUID, toUID, clientMsgNo, contentText string, payload []byte) (int64, int64, error) {
	if strings.TrimSpace(c.config.BaseURL) == "" {
		return 0, 0, errors.New("wukong base url is empty")
	}

	// 文本消息不能直接把原始字符串做 base64。
	// WuKongIM 需要的 payload 是“消息协议 JSON”整体做 base64。
	encodedPayload, err := c.encodeMessagePayload(contentText, payload)
	if err != nil {
		return 0, 0, err
	}

	// 普通点对点私聊直接用对方 uid 作为 channel_id。
	// 这里不再附带 subscribers，避免被服务端判成非法发送请求。
	resp, err := c.post(ctx, c.messageSendPath(), map[string]any{
		"from_uid":      fromUID,
		"channel_id":    toUID,
		"channel_type":  1,
		"client_msg_no": clientMsgNo,
		"payload":       encodedPayload,
	})
	if err != nil {
		return 0, 0, err
	}
	return extractInt64(resp, "message_id"), extractInt64(resp, "message_seq"), nil
}

func (c *Client) MarkConversationRead(ctx context.Context, uid, channelID string, channelType int32, messageSeq int64) error {
	if strings.TrimSpace(c.config.BaseURL) == "" {
		return errors.New("wukong base url is empty")
	}

	// 把指定会话的已读位置同步给 WuKongIM。
	_, err := c.post(ctx, c.clearUnreadPath(), map[string]any{
		"uid":          uid,
		"channel_id":   channelID,
		"channel_type": channelType,
		"message_seq":  messageSeq,
	})
	return err
}

func (c *Client) getRoute(ctx context.Context, uid, token string, deviceFlag, deviceLevel int32) (RouteInfo, error) {
	resp, err := c.post(ctx, c.routePath(), map[string]any{
		"uid":          uid,
		"token":        token,
		"intranet":     c.config.RouteIntranet,
		"device_flag":  deviceFlag,
		"device_level": deviceLevel,
	})
	if err != nil {
		return RouteInfo{}, err
	}
	return RouteInfo{
		TCPAddr: firstNonEmpty(extractString(resp, "tcp_addr"), c.config.PublicTCPAddr),
		WSAddr:  firstNonEmpty(extractString(resp, "ws_addr"), c.config.PublicWSAddr),
		WSSAddr: firstNonEmpty(extractString(resp, "wss_addr"), c.config.PublicWSSAddr),
	}, nil
}

func (c *Client) encodeMessagePayload(contentText string, payload []byte) (string, error) {
	if len(payload) > 0 {
		return base64.StdEncoding.EncodeToString(payload), nil
	}

	// 当前私聊文本消息按 WuKongIM 文本协议包装。
	messagePayload := map[string]any{
		"type":    1,
		"content": contentText,
	}
	raw, err := json.Marshal(messagePayload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (c *Client) tokenUpdatePath() string {
	return defaultPath(c.config.TokenUpdatePath, "/user/token/update")
}

func (c *Client) routePath() string {
	return defaultPath(c.config.RoutePath, "/route")
}

func (c *Client) messageSendPath() string {
	return defaultPath(c.config.MessageSendPath, "/message/send")
}

func (c *Client) clearUnreadPath() string {
	return defaultPath(c.config.ConversationClearUnreadPath, "/conversation/clearUnread")
}

func defaultPath(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

func (c *Client) post(ctx context.Context, path string, body map[string]any) (map[string]any, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.config.BaseURL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(c.config.AdminToken); token != "" {
		// 如果部署的 WuKongIM 管理接口启用了鉴权，这里统一附带管理 token。
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wukong http status=%d body=%s", resp.StatusCode, string(respBytes))
	}

	var result map[string]any
	if len(respBytes) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, err
	}

	if code, ok := result["code"]; ok {
		switch typed := code.(type) {
		case float64:
			if int64(typed) != 0 {
				return nil, fmt.Errorf("wukong error: %v", result)
			}
		case int64:
			if typed != 0 {
				return nil, fmt.Errorf("wukong error: %v", result)
			}
		}
	}

	if data, ok := result["data"].(map[string]any); ok {
		return data, nil
	}
	return result, nil
}

func extractString(data map[string]any, key string) string {
	if value, ok := data[key]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func extractInt64(data map[string]any, key string) int64 {
	if value, ok := data[key]; ok {
		switch typed := value.(type) {
		case float64:
			return int64(typed)
		case int64:
			return typed
		case int:
			return int64(typed)
		}
	}
	return 0
}
