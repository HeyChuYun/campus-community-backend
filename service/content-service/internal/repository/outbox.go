package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type OutboxEvent struct {
	EventID     string
	EventType   string
	RoutingKey  string
	PayloadJSON string
}

type OutboxStore interface {
	Create(ctx context.Context, event OutboxEvent) error
}

type contentOutboxStore struct {
	db *gorm.DB
}

type contentOutboxEventRecord struct {
	EventID     string     `gorm:"column:event_id"`
	EventType   string     `gorm:"column:event_type"`
	RoutingKey  string     `gorm:"column:routing_key"`
	PayloadJSON string     `gorm:"column:payload_json"`
	Status      int        `gorm:"column:status"`
	RetryCount  int        `gorm:"column:retry_count"`
	NextRetryAt *time.Time `gorm:"column:next_retry_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	SentAt      *time.Time `gorm:"column:sent_at"`
}

func (contentOutboxEventRecord) TableName() string {
	return "content_outbox_events"
}

func NewOutboxStore(db *gorm.DB) OutboxStore {
	return &contentOutboxStore{db: requireGormDB(db)}
}

func (s *contentOutboxStore) Create(ctx context.Context, event OutboxEvent) error {
	return s.db.WithContext(ctx).Create(&contentOutboxEventRecord{
		EventID:     event.EventID,
		EventType:   event.EventType,
		RoutingKey:  event.RoutingKey,
		PayloadJSON: event.PayloadJSON,
		Status:      1,
		RetryCount:  0,
		CreatedAt:   time.Now(),
	}).Error
}
