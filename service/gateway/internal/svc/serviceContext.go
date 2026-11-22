package svc

import (
	"strings"

	"hfjj/backend/captcha-solver-service/captchasolverservice"
	"hfjj/backend/gateway/internal/backoffice"
	"hfjj/backend/gateway/internal/commercial"
	"hfjj/backend/gateway/internal/config"
	"hfjj/backend/gateway/internal/moderation"
	"hfjj/backend/gateway/internal/privacy"
	"hfjj/backend/gateway/internal/reporting"
	"hfjj/backend/gateway/internal/updateclient"
	"hfjj/backend/gateway/internal/userquery"
	"hfjj/backend/passport-service/passportservice"
	"hfjj/backend/pkg/rpcx"
	"hfjj/backend/proto/contentpb"
	"hfjj/backend/proto/impb"
	"hfjj/backend/proto/relationpb"
	"hfjj/backend/proto/updatepb"
	"hfjj/backend/school-service/schoolservice"

	"github.com/redis/go-redis/v9"
	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config      config.Config
	GormDB      *gorm.DB
	Redis       *redis.Client
	Replay      ReplayProtector
	Realtime    *RealtimeHub
	Commercial  *commercial.Service
	Privacy     *privacy.Service
	Reporting   *reporting.Service
	Moderation  *moderation.Service
	Backoffice  *backoffice.Service
	Update      *updateclient.Client
	UserQuery   *userquery.Service
	PassportRPC passportservice.PassportService
	SchoolRPC   schoolservice.SchoolService
	CaptchaRPC  captchasolverservice.CaptchaSolverService
	RelationRPC relationpb.RelationServiceClient
	ContentRPC  contentpb.ContentServiceClient
	ImRPC       impb.IMServiceClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	redisClient := newRedisClient(c.Redis)
	gormDB := newGormPostgresClient(c.Postgres.DSN)

	prefix := c.Redis.Key
	if prefix == "" {
		prefix = "gateway:"
	}

	// 统一通过项目兼容 RPC 客户端接入 etcd 服务发现，规避 go-zero 原生 resolver 的 bad resolver state 问题。
	passportClient := rpcx.MustNewClient(c.PassportRpc)
	schoolClient := rpcx.MustNewClient(c.SchoolRpc)
	captchaClient := rpcx.MustNewClient(c.CaptchaRpc)
	relationClient := rpcx.MustNewClient(c.RelationRpc)
	contentClient := rpcx.MustNewClient(c.ContentRpc)
	imClient := rpcx.MustNewClient(c.ImRpc)
	updateClient := rpcx.MustNewClient(c.UpdateRpc)
	contentRPC := contentpb.NewContentServiceClient(contentClient.Conn())
	commercialService := commercial.NewService(
		commercial.NewGormRepository(gormDB),
		contentRPC,
		commercial.NewPaymentRegistry(c.Payment),
		commercial.NewCache(redisClient, prefix),
		commercial.NewEventPublisher(c.RabbitMQ),
		commercial.NewOptions(c.Commercial),
	)
	privacyService := privacy.NewService(privacy.NewGormRepository(gormDB))
	reportingService := reporting.NewService(reporting.NewGormRepository(gormDB))
	moderationService := moderation.NewService(moderation.NewGormRepository(gormDB))
	backofficeService := backoffice.NewService(
		backoffice.NewGormRepository(gormDB),
		redisClient,
		prefix,
		backoffice.NewRBACService(gormDB, redisClient, prefix),
	)
	userQueryService := userquery.NewService(gormDB)

	return &ServiceContext{
		Config:      c,
		GormDB:      gormDB,
		Redis:       redisClient,
		Replay:      NewReplayProtector(redisClient, prefix),
		Realtime:    NewRealtimeHub(),
		Commercial:  commercialService,
		Privacy:     privacyService,
		Reporting:   reportingService,
		Moderation:  moderationService,
		Backoffice:  backofficeService,
		Update:      updateclient.New(updatepb.NewUpdateServiceClient(updateClient.Conn())),
		UserQuery:   userQueryService,
		PassportRPC: passportservice.NewPassportService(passportClient),
		SchoolRPC:   schoolservice.NewSchoolService(schoolClient),
		CaptchaRPC:  captchasolverservice.NewCaptchaSolverService(captchaClient),
		RelationRPC: relationpb.NewRelationServiceClient(relationClient.Conn()),
		ContentRPC:  contentRPC,
		ImRPC:       impb.NewIMServiceClient(imClient.Conn()),
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
		panic("gateway redis host is empty")
	}
	return redis.NewClient(&redis.Options{
		Addr:      strings.TrimSpace(strings.Split(cfg.Host, ",")[0]),
		Username:  cfg.User,
		Password:  cfg.Pass,
		DB:        0,
		TLSConfig: nil,
	})
}
