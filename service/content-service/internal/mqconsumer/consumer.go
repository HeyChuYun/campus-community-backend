package mqconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"hfjj/backend/content-service/internal/logic"
	"hfjj/backend/content-service/internal/svc"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type Consumer struct {
	svcCtx *svc.ServiceContext

	mu     sync.Mutex
	conn   *amqp.Connection
	ch     *amqp.Channel
	cancel context.CancelFunc
	done   chan struct{}
}

type domainEvent struct {
	EventID      string `json:"event_id"`
	EventType    string `json:"event_type"`
	ResourceType string `json:"resource_type"`
	ResourceID   int64  `json:"resource_id"`
}

func NewConsumer(svcCtx *svc.ServiceContext) *Consumer {
	return &Consumer{
		svcCtx: svcCtx,
		done:   make(chan struct{}),
	}
}

func (c *Consumer) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cfg := c.svcCtx.Config
	if !cfg.Moderation.Enabled || !cfg.Moderation.ReviewQueue.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.RabbitMQ.URL) == "" {
		return fmt.Errorf("rabbitmq url is empty")
	}

	conn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	if err := ch.ExchangeDeclare(
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.ExchangeType,
		cfg.RabbitMQ.Durable,
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
		cfg.Moderation.ReviewQueue.Queue,
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

	if cfg.Moderation.ReviewQueue.PrefetchCount > 0 {
		if err := ch.Qos(cfg.Moderation.ReviewQueue.PrefetchCount, 0, false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
	}

	for _, binding := range cfg.Moderation.ReviewQueue.Bindings {
		if err := ch.QueueBind(
			cfg.Moderation.ReviewQueue.Queue,
			binding,
			cfg.RabbitMQ.Exchange,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	msgs, err := ch.Consume(
		cfg.Moderation.ReviewQueue.Queue,
		cfg.Moderation.ReviewQueue.ConsumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		cancel()
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	c.cancel = cancel
	c.conn = conn
	c.ch = ch
	logx.Infof(
		"content moderation consumer started: queue=%s consumerTag=%s bindings=%s exchange=%s",
		cfg.Moderation.ReviewQueue.Queue,
		cfg.Moderation.ReviewQueue.ConsumerTag,
		strings.Join(cfg.Moderation.ReviewQueue.Bindings, ","),
		cfg.RabbitMQ.Exchange,
	)
	go c.loop(ctx, msgs)
	return nil
}

func (c *Consumer) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	ch := c.ch
	conn := c.conn
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
	logx.Info("content moderation consumer stopped")
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
				logx.WithContext(ctx).Errorf("content moderation consume failed: routingKey=%s err=%v", msg.RoutingKey, err)
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
		"content moderation consumer received event: eventId=%s eventType=%s resourceType=%s resourceId=%d routingKey=%s",
		event.EventID,
		event.EventType,
		event.ResourceType,
		event.ResourceID,
		msg.RoutingKey,
	)
	if event.EventType != "content.post.created" || event.ResourceType != "post" || event.ResourceID <= 0 {
		logx.WithContext(ctx).Infof(
			"content moderation consumer skipped event: eventId=%s eventType=%s resourceType=%s resourceId=%d",
			event.EventID,
			event.EventType,
			event.ResourceType,
			event.ResourceID,
		)
		return nil
	}
	return logic.ProcessPostModeration(ctx, c.svcCtx, event.ResourceID)
}
