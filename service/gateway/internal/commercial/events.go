package commercial

import (
	"context"
	"encoding/json"
	"time"

	"hfjj/backend/pkg/outboxmq"

	"github.com/zeromicro/go-zero/core/logx"
)

type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, payload any)
	Close() error
}

type mqEventPublisher struct {
	publisher outboxmq.Publisher
}

func NewEventPublisher(cfg outboxmq.RabbitMQConfig) EventPublisher {
	if !cfg.Enabled || cfg.URL == "" {
		return noopEventPublisher{}
	}
	return &mqEventPublisher{publisher: outboxmq.NewRabbitMQPublisher(cfg)}
}

func (p *mqEventPublisher) Publish(ctx context.Context, routingKey string, payload any) {
	if p == nil || p.publisher == nil {
		return
	}
	body, err := json.Marshal(map[string]any{
		"event_id":    NewOrderNo("EV"),
		"routing_key": routingKey,
		"payload":     payload,
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("marshal commercial event failed: %v", err)
		return
	}
	if err := p.publisher.Publish(ctx, routingKey, body); err != nil {
		logx.WithContext(ctx).Errorf("publish commercial event failed: routingKey=%s err=%v", routingKey, err)
	}
}

func (p *mqEventPublisher) Close() error {
	if p == nil || p.publisher == nil {
		return nil
	}
	return p.publisher.Close()
}

type noopEventPublisher struct{}

func (noopEventPublisher) Publish(context.Context, string, any) {}

func (noopEventPublisher) Close() error { return nil }
