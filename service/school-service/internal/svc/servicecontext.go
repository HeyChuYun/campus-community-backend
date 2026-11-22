package svc

import (
	"strings"

	"hfjj/backend/captcha-solver-service/captchasolverservice"
	"hfjj/backend/pkg/rpcx"
	"hfjj/backend/school-service/internal/config"
	"hfjj/backend/school-service/internal/repository"

	"github.com/redis/go-redis/v9"
	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config             config.Config
	GormDB             *gorm.DB
	Redis              *redis.Client
	RedisPrefix        string
	AuthStore          repository.AuthStore
	AuthorizedStore    repository.AuthorizedStore
	SchoolAccountStore repository.SchoolAccountStore
	SchoolSessionStore repository.SchoolSessionStore
	CampusClient       *repository.CampusClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	gormDB := newGormPostgresClient(c.Postgres.DSN)
	rdb := newRedisClient(c.AppRedis)
	// 验证码服务客户端走项目兼容 resolver，保留 etcd 动态发现。
	captchaClient := rpcx.MustNewClient(c.CaptchaRpc)
	prefix := c.AppRedis.Key
	if prefix == "" {
		prefix = "school:"
	}

	return &ServiceContext{
		Config:             c,
		GormDB:             gormDB,
		Redis:              rdb,
		RedisPrefix:        prefix,
		AuthStore:          repository.NewAuthStore(rdb, prefix),
		AuthorizedStore:    repository.NewAuthorizedStore(rdb, prefix),
		SchoolAccountStore: repository.NewSchoolAccountStore(gormDB),
		SchoolSessionStore: repository.NewSchoolSessionStore(rdb, prefix),
		CampusClient:       repository.NewCampusClient(c.Campus, captchasolverservice.NewCaptchaSolverService(captchaClient)),
	}
}

func newGormPostgresClient(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}

func newRedisClient(cfg gozeroredis.RedisKeyConf) *redis.Client {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:      strings.TrimSpace(strings.Split(cfg.Host, ",")[0]),
		Username:  cfg.User,
		Password:  cfg.Pass,
		DB:        0,
		TLSConfig: nil,
	})
}
