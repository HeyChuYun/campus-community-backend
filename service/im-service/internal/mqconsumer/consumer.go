package mqconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"hfjj/backend/im-service/internal/config"
	"hfjj/backend/im-service/internal/repository"
	"hfjj/backend/im-service/internal/wukong"
	"hfjj/backend/proto/relationpb"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type Consumer struct {
	config      config.Config
	repo        *repository.Repository
	relationRPC relationpb.RelationServiceClient
	wkClient    *wukong.Client

	mu     sync.Mutex
	conn   *amqp.Connection
	ch     *amqp.Channel
	cancel context.CancelFunc
	done   chan struct{}
}

type domainEvent struct {
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	OccurredAt     string          `json:"occurred_at"`
	Producer       string          `json:"producer"`
	TraceID        string          `json:"trace_id"`
	OperatorUserID int64           `json:"operator_user_id"`
	TargetUserID   int64           `json:"target_user_id"`
	ResourceType   string          `json:"resource_type"`
	ResourceID     int64           `json:"resource_id"`
	Payload        json.RawMessage `json:"payload"`
}

func NewConsumer(cfg config.Config, repo *repository.Repository, relationRPC relationpb.RelationServiceClient, wkClient *wukong.Client) *Consumer {
	return &Consumer{
		config:      cfg,
		repo:        repo,
		relationRPC: relationRPC,
		wkClient:    wkClient,
		done:        make(chan struct{}),
	}
}

func (c *Consumer) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.TrimSpace(c.config.RabbitMQ.URL) == "" {
		return fmt.Errorf("rabbitmq url is empty")
	}
	conn, err := amqp.Dial(c.config.RabbitMQ.URL)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	if err := ch.ExchangeDeclare(
		c.config.RabbitMQ.Exchange,
		c.config.RabbitMQ.ExchangeType,
		c.config.RabbitMQ.Durable,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	if _, err := ch.QueueDeclare(
		c.config.Consumer.Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	if c.config.Consumer.PrefetchCount > 0 {
		if err := ch.Qos(c.config.Consumer.PrefetchCount, 0, false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
	}

	for _, binding := range c.config.Consumer.Bindings {
		if err := ch.QueueBind(
			c.config.Consumer.Queue,
			binding,
			c.config.RabbitMQ.Exchange,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.conn = conn
	c.ch = ch

	msgs, err := ch.Consume(
		c.config.Consumer.Queue,
		c.config.Consumer.ConsumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	logx.Infof(
		"im interaction consumer started: queue=%s consumerTag=%s bindings=%s exchange=%s",
		c.config.Consumer.Queue,
		c.config.Consumer.ConsumerTag,
		strings.Join(c.config.Consumer.Bindings, ","),
		c.config.RabbitMQ.Exchange,
	)
	go c.loop(ctx, msgs)
	return nil
}

func (c *Consumer) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	conn := c.conn
	ch := c.ch
	done := c.done
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ch != nil {
		_ = ch.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
	<-done
	logx.Info("im interaction consumer stopped")
}

func (c *Consumer) loop(ctx context.Context, msgs <-chan amqp.Delivery) {
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			if err := c.handleMessage(ctx, msg); err != nil {
				logx.WithContext(ctx).Errorf("im mq consume failed: routingKey=%s err=%v", msg.RoutingKey, err)
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery) error {
	var event domainEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return err
	}
	logx.WithContext(ctx).Infof(
		"im interaction consumer received event: eventId=%s eventType=%s targetUserId=%d resourceType=%s resourceId=%d routingKey=%s",
		event.EventID,
		event.EventType,
		event.TargetUserID,
		event.ResourceType,
		event.ResourceID,
		msg.RoutingKey,
	)
	if strings.TrimSpace(event.EventID) == "" {
		return fmt.Errorf("event_id is empty")
	}

	already, err := c.repo.HasConsumedEvent(ctx, event.EventID, "im-service")
	if err != nil {
		return err
	}
	if already {
		return nil
	}

	if c.isSystemNotificationEvent(event.EventType) {
		if err := c.handleSystemNotificationEvent(ctx, event); err != nil {
			return err
		}
		return c.repo.MarkConsumedEvent(ctx, event.EventID, "im-service", "done")
	}

	title, content, eventTypeCode := c.buildInteractionText(ctx, event)
	if event.TargetUserID <= 0 {
		return nil
	}

	operator, _ := c.repo.GetUserBasic(ctx, event.OperatorUserID)
	interaction := repository.InteractionMessage{
		UserID:            event.TargetUserID,
		EventID:           event.EventID,
		EventType:         eventTypeCode,
		OperatorUserID:    event.OperatorUserID,
		OperatorNickname:  operator.Nickname,
		OperatorAvatarURL: operator.AvatarURL,
		Title:             title,
		Content:           content,
		ResourceType:      event.ResourceType,
		ResourceID:        event.ResourceID,
		IsRead:            false,
		CreatedAt:         time.Now(),
	}
	if err := c.repo.CreateInteractionMessage(ctx, interaction); err != nil {
		return err
	}

	if c.wkClient != nil && strings.TrimSpace(c.config.WuKong.SystemUID) != "" {
		targetIMUID := wukong.FormatIMUID(event.TargetUserID)
		payload, _ := json.Marshal(map[string]any{
			"biz_type":         "interaction",
			"event_type":       event.EventType,
			"title":            title,
			"content":          content,
			"resource_type":    event.ResourceType,
			"resource_id":      event.ResourceID,
			"operator_user_id": event.OperatorUserID,
		})
		_, _, _ = c.wkClient.SendDirectMessage(ctx, c.config.WuKong.SystemUID, targetIMUID, event.EventID, content, payload)
	}

	return c.repo.MarkConsumedEvent(ctx, event.EventID, "im-service", "done")
}

func (c *Consumer) isSystemNotificationEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "content.post.blocked":
		return true
	default:
		return false
	}
}

func (c *Consumer) handleSystemNotificationEvent(ctx context.Context, event domainEvent) error {
	if event.TargetUserID <= 0 {
		return nil
	}

	title, content := c.buildSystemNotificationText(event)
	notification := repository.SystemNotification{
		UserID:           event.TargetUserID,
		EventID:          event.EventID,
		NoticeType:       strings.TrimSpace(event.EventType),
		Title:            title,
		Content:          content,
		ResourceType:     event.ResourceType,
		ResourceID:       event.ResourceID,
		MediaPayloadJSON: "",
		IsRead:           false,
		CreatedAt:        time.Now(),
	}
	return c.repo.CreateSystemNotification(ctx, notification)
}

func (c *Consumer) buildInteractionText(ctx context.Context, event domainEvent) (string, string, int64) {
	operator, _ := c.repo.GetUserBasic(ctx, event.OperatorUserID)
	name := strings.TrimSpace(operator.Nickname)
	if name == "" {
		if event.OperatorUserID > 0 {
			name = fmt.Sprintf("user %d", event.OperatorUserID)
		} else {
			name = "system"
		}
	}

	switch event.EventType {
	case "content.post.liked":
		return "收到点赞", fmt.Sprintf("%s 赞了你的帖子", name), 1
	case "content.post.commented":
		return "收到评论", fmt.Sprintf("%s 评论了你的帖子", name), 2
	case "content.post.favorited":
		return "收到收藏", fmt.Sprintf("%s 收藏了你的帖子", name), 3
	case "relation.user.followed":
		return "新增关注者", fmt.Sprintf("%s 关注了你", name), 4
	case "relation.user.unfollowed":
		return "对方已取关", fmt.Sprintf("%s 取消关注了你", name), 5
	default:
		return "新互动", fmt.Sprintf("%s 触发了一条新互动", name), 0
	}
}

func (c *Consumer) buildSystemNotificationText(event domainEvent) (string, string) {
	switch strings.TrimSpace(event.EventType) {
	case "content.post.blocked":
		return buildPostBlockedSystemNotificationMessage(event.Payload)
	default:
		return "\u7cfb\u7edf\u901a\u77e5", "\u4f60\u6709\u4e00\u6761\u65b0\u7684\u7cfb\u7edf\u901a\u77e5"
	}
}

func buildPostBlockedSystemNotificationMessage(payload json.RawMessage) (string, string) {
	type blockedPayload struct {
		Categories          []string `json:"categories"`
		Reason              string   `json:"reason"`
		MuteScope           string   `json:"mute_scope"`
		MuteDurationSeconds int64    `json:"mute_duration_seconds"`
		ViolationNo         string   `json:"violation_no"`
	}

	var body blockedPayload
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &body)
	}

	parts := make([]string, 0, 4)
	if len(body.Categories) > 0 {
		parts = append(parts, "\u8fdd\u89c4\u7c7b\u578b\uff1a"+strings.Join(body.Categories, "\u3001"))
	}
	if strings.TrimSpace(body.Reason) != "" {
		parts = append(parts, "\u5904\u7406\u539f\u56e0\uff1a"+strings.TrimSpace(body.Reason))
	}
	if strings.TrimSpace(body.MuteScope) == "post_publish" && body.MuteDurationSeconds > 0 {
		parts = append(parts, "\u53d1\u5e16\u7981\u8a00\uff1a"+formatDurationTextCN(body.MuteDurationSeconds))
	}
	if strings.TrimSpace(body.ViolationNo) != "" {
		parts = append(parts, "\u7f16\u53f7\uff1a"+strings.TrimSpace(body.ViolationNo))
	}

	content := "\u4f60\u53d1\u5e03\u7684\u5e16\u5b50\u56e0\u8fdd\u53cd\u793e\u533a\u5185\u5bb9\u89c4\u8303\uff0c\u5df2\u88ab\u9690\u85cf\u5904\u7406\u3002"
	if len(parts) > 0 {
		content += "\n" + strings.Join(parts, "\n")
	}
	content += "\n\u5982\u5bf9\u5904\u7406\u7ed3\u679c\u6709\u5f02\u8bae\uff0c\u53ef\u4ee5\u5728\u901a\u77e5\u9875\u67e5\u770b\u8be6\u60c5\u3002"
	return "\u5e16\u5b50\u5df2\u88ab\u5904\u7406", content
}

func formatDurationTextCN(seconds int64) string {
	if seconds <= 0 {
		return "0\u5206\u949f"
	}
	if seconds%86400 == 0 {
		return fmt.Sprintf("%d\u5929", seconds/86400)
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%d\u5c0f\u65f6", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d\u5206\u949f", seconds/60)
	}
	return fmt.Sprintf("%d\u79d2", seconds)
}

func buildPostBlockedMessage(payload json.RawMessage) (string, string) {
	type blockedPayload struct {
		Categories          []string `json:"categories"`
		Reason              string   `json:"reason"`
		MuteScope           string   `json:"mute_scope"`
		MuteDurationSeconds int64    `json:"mute_duration_seconds"`
		ViolationNo         string   `json:"violation_no"`
	}

	var body blockedPayload
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &body)
	}

	parts := make([]string, 0, 4)
	if len(body.Categories) > 0 {
		parts = append(parts, "违规分类："+strings.Join(body.Categories, "、"))
	}
	if strings.TrimSpace(body.Reason) != "" {
		parts = append(parts, "处理原因："+strings.TrimSpace(body.Reason))
	}
	if strings.TrimSpace(body.MuteScope) == "post_publish" && body.MuteDurationSeconds > 0 {
		parts = append(parts, "发帖禁言："+formatDurationText(body.MuteDurationSeconds))
	}
	if strings.TrimSpace(body.ViolationNo) != "" {
		parts = append(parts, "编号："+strings.TrimSpace(body.ViolationNo))
	}

	content := "你发布的帖子因违反社区内容规范，已被隐藏处理。"
	if len(parts) > 0 {
		content += "\n" + strings.Join(parts, "\n")
	}
	content += "\n如对处理结果有异议，可在互动消息中进入申诉入口并补充说明。"
	return "帖子已被处理", content
}

func formatDurationText(seconds int64) string {
	if seconds <= 0 {
		return "0分钟"
	}
	if seconds%86400 == 0 {
		return fmt.Sprintf("%d天", seconds/86400)
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%d小时", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d分钟", seconds/60)
	}
	return fmt.Sprintf("%d秒", seconds)
}
