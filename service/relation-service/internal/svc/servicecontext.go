package svc

import (
	"hfjj/backend/pkg/outboxmq"
	"hfjj/backend/relation-service/internal/config"
	"hfjj/backend/relation-service/internal/repository"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config        config.Config
	GormDB        *gorm.DB
	RelationStore repository.RelationStore
	OutboxStore   repository.OutboxStore
	Dispatcher    *outboxmq.Dispatcher
}

func NewServiceContext(c config.Config) *ServiceContext {
	gormDB := newGormPostgresClient(c.Postgres.DSN)
	return &ServiceContext{
		Config:        c,
		GormDB:        gormDB,
		RelationStore: repository.NewRelationStore(gormDB),
		OutboxStore:   repository.NewOutboxStore(gormDB),
		Dispatcher:    outboxmq.NewDispatcher(gormDB, "relation_outbox_events", "relation-service", c.Outbox, c.RabbitMQ),
	}
}

func newGormPostgresClient(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}
