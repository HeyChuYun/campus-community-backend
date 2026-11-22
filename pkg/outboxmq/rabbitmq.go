package outboxmq

import (
	"context"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitMQPublisher struct {
	config RabbitMQConfig

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbitMQPublisher(cfg RabbitMQConfig) Publisher {
	if strings.TrimSpace(cfg.Exchange) == "" {
		cfg.Exchange = "hfjj.biz.event"
	}
	if strings.TrimSpace(cfg.ExchangeType) == "" {
		cfg.ExchangeType = "topic"
	}
	return &rabbitMQPublisher{config: cfg}
}

// Publish 负责确保连接与 exchange 可用，并把事件体投递到指定 routing key。
func (p *rabbitMQPublisher) Publish(ctx context.Context, routingKey string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureChannel(); err != nil {
		return err
	}

	err := p.ch.PublishWithContext(ctx, p.config.Exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         payload,
		DeliveryMode: amqp.Persistent,
	})
	if err != nil {
		_ = p.closeLocked()
		return err
	}
	return nil
}

func (p *rabbitMQPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

func (p *rabbitMQPublisher) ensureChannel() error {
	if p.ch != nil && !p.ch.IsClosed() && p.conn != nil && !p.conn.IsClosed() {
		return nil
	}
	if err := p.closeLocked(); err != nil {
		return err
	}

	conn, err := amqp.Dial(p.config.URL)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	if err := ch.ExchangeDeclare(
		p.config.Exchange,
		p.config.ExchangeType,
		p.config.Durable,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	p.conn = conn
	p.ch = ch
	return nil
}

func (p *rabbitMQPublisher) closeLocked() error {
	var err error
	if p.ch != nil {
		if closeErr := p.ch.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		p.ch = nil
	}
	if p.conn != nil {
		if closeErr := p.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		p.conn = nil
	}
	return err
}
