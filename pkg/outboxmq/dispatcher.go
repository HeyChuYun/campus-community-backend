package outboxmq

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RabbitMQConfig struct {
	Enabled      bool
	URL          string
	Exchange     string
	ExchangeType string
	Durable      bool
}

type DispatcherConfig struct {
	Enabled      bool
	PollInterval time.Duration
	BatchSize    int
	MaxRetry     int
	RetryBackoff time.Duration
}

type Event struct {
	ID         int64
	EventID    string
	EventType  string
	RoutingKey string
	Payload    []byte
	RetryCount int
}

const (
	outboxStatusPending = 1
	outboxStatusSent    = 2
	outboxStatusFailed  = 3
)

type outboxRow struct {
	ID          int64      `gorm:"column:id"`
	EventID     string     `gorm:"column:event_id"`
	EventType   string     `gorm:"column:event_type"`
	RoutingKey  string     `gorm:"column:routing_key"`
	PayloadJSON []byte     `gorm:"column:payload_json"`
	Status      int64      `gorm:"column:status"`
	RetryCount  int        `gorm:"column:retry_count"`
	NextRetryAt *time.Time `gorm:"column:next_retry_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	SentAt      *time.Time `gorm:"column:sent_at"`
}

type Publisher interface {
	Publish(ctx context.Context, routingKey string, payload []byte) error
	Close() error
}

type Dispatcher struct {
	db        *gorm.DB
	table     string
	name      string
	config    DispatcherConfig
	publisher Publisher

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	done    chan struct{}
}

func NewDispatcher(db *gorm.DB, table string, name string, cfg DispatcherConfig, mqCfg RabbitMQConfig) *Dispatcher {
	if db == nil || !cfg.Enabled || strings.TrimSpace(table) == "" || !mqCfg.Enabled || strings.TrimSpace(mqCfg.URL) == "" {
		return nil
	}
	normalizeDispatcherConfig(&cfg)
	return &Dispatcher{
		db:        db,
		table:     table,
		name:      name,
		config:    cfg,
		publisher: NewRabbitMQPublisher(mqCfg),
	}
}

// Start starts the background outbox dispatcher loop.
func (d *Dispatcher) Start() {
	if d == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.done = make(chan struct{})
	d.running = true
	logx.Infof("%s outbox dispatcher started: table=%s pollInterval=%s batchSize=%d", d.name, d.table, d.config.PollInterval, d.config.BatchSize)
	go d.loop(ctx, d.done)
}

// Stop stops the background outbox dispatcher loop.
func (d *Dispatcher) Stop() {
	if d == nil {
		return
	}

	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	cancel := d.cancel
	done := d.done
	d.running = false
	d.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	<-done
	if d.publisher != nil {
		_ = d.publisher.Close()
	}
	logx.Infof("%s outbox dispatcher stopped", d.name)
}

func (d *Dispatcher) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := d.dispatchOnce(ctx); err != nil {
			logx.WithContext(ctx).Errorf("%s outbox dispatch failed: %v", d.name, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) dispatchOnce(ctx context.Context) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		events, err := d.lockPendingEvents(ctx, tx)
		if err != nil {
			return err
		}
		if len(events) > 0 {
			logx.WithContext(ctx).Infof("%s outbox picked pending events: count=%d", d.name, len(events))
		}

		for _, event := range events {
			if err := d.publisher.Publish(ctx, event.RoutingKey, event.Payload); err != nil {
				if updateErr := d.markFailed(ctx, tx, event, err); updateErr != nil {
					return updateErr
				}
				logx.WithContext(ctx).Errorf("%s publish failed: eventId=%s eventType=%s err=%v", d.name, event.EventID, event.EventType, err)
				continue
			}
			if err := d.markSent(ctx, tx, event.ID); err != nil {
				return err
			}
			logx.WithContext(ctx).Infof(
				"%s outbox published event: dbId=%d eventId=%s eventType=%s routingKey=%s",
				d.name,
				event.ID,
				event.EventID,
				event.EventType,
				event.RoutingKey,
			)
		}

		return nil
	})
}

func (d *Dispatcher) lockPendingEvents(ctx context.Context, tx *gorm.DB) ([]Event, error) {
	rows := make([]outboxRow, 0, d.config.BatchSize)
	err := tx.WithContext(ctx).
		Table(d.table).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = ?", outboxStatusPending).
		Or("(status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= now())", outboxStatusFailed).
		Order("created_at ASC").
		Limit(d.config.BatchSize).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, Event{
			ID:         row.ID,
			EventID:    row.EventID,
			EventType:  row.EventType,
			RoutingKey: row.RoutingKey,
			Payload:    row.PayloadJSON,
			RetryCount: row.RetryCount,
		})
	}
	return events, nil
}

func (d *Dispatcher) markSent(ctx context.Context, tx *gorm.DB, id int64) error {
	return tx.WithContext(ctx).
		Table(d.table).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        outboxStatusSent,
			"sent_at":       gorm.Expr("now()"),
			"next_retry_at": nil,
		}).Error
}

func (d *Dispatcher) markFailed(ctx context.Context, tx *gorm.DB, event Event, publishErr error) error {
	nextRetryCount := event.RetryCount + 1
	var nextRetryAt any
	if nextRetryCount < d.config.MaxRetry {
		nextRetryAt = time.Now().Add(d.config.RetryBackoff * time.Duration(nextRetryCount))
	}

	return tx.WithContext(ctx).
		Table(d.table).
		Where("id = ?", event.ID).
		Updates(map[string]any{
			"status":        outboxStatusFailed,
			"retry_count":   nextRetryCount,
			"next_retry_at": nextRetryAt,
		}).Error
}

func normalizeDispatcherConfig(cfg *DispatcherConfig) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 3 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}
	if cfg.MaxRetry <= 0 {
		cfg.MaxRetry = 10
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 10 * time.Second
	}
}
