package svc

import (
	"hfjj/backend/update-service/internal/appversion"
	"hfjj/backend/update-service/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config     config.Config
	GormDB     *gorm.DB
	AppVersion *appversion.Service
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := newGormPostgresClient(c.Postgres.DSN)
	return &ServiceContext{
		Config:     c,
		GormDB:     db,
		AppVersion: appversion.NewService(db),
	}
}

func newGormPostgresClient(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}
