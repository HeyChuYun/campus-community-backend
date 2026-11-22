package svc

import (
	"strings"

	"hfjj/backend/content-service/internal/config"
	"hfjj/backend/pkg/rpcx"
	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/pkg/outboxmq"
	"hfjj/backend/proto/relationpb"

	"github.com/redis/go-redis/v9"
	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config          config.Config
	GormDB          *gorm.DB
	Redis           *redis.Client
	PostStore       repository.PostStore
	ModerationStore repository.ModerationStore
	MembershipStore repository.MembershipStore
	TopicStore      repository.TopicStore
	CommentStore    repository.CommentStore
	LikeStore       repository.PostLikeStore
	FavStore        repository.PostFavoriteStore
	OutboxStore     repository.OutboxStore
	RelationRPC     relationpb.RelationServiceClient
	Dispatcher      *outboxmq.Dispatcher
}

func NewServiceContext(c config.Config) *ServiceContext {
	gormDB := newGormPostgresClient(c.Postgres.DSN)
	redisClient := newRedisClient(c.AppRedis)

	var relationClient relationpb.RelationServiceClient
	if hasRPCClientTarget(c.RelationRpc) {
		// 关系服务客户端继续走 etcd 发现，但使用项目兼容 resolver。
		relationClient = relationpb.NewRelationServiceClient(rpcx.MustNewClient(c.RelationRpc).Conn())
	}

	return &ServiceContext{
		Config:          c,
		GormDB:          gormDB,
		Redis:           redisClient,
		PostStore:       repository.NewPostStore(gormDB),
		ModerationStore: repository.NewModerationStore(gormDB),
		MembershipStore: repository.NewMembershipStore(gormDB),
		TopicStore:      repository.NewTopicStore(gormDB),
		CommentStore:    repository.NewCommentStore(gormDB),
		LikeStore:       repository.NewPostLikeStore(gormDB),
		FavStore:        repository.NewPostFavoriteStore(gormDB),
		OutboxStore:     repository.NewOutboxStore(gormDB),
		RelationRPC:     relationClient,
		Dispatcher:      outboxmq.NewDispatcher(gormDB, "content_outbox_events", "content-service", c.Outbox, c.RabbitMQ),
	}
}

func newGormPostgresClient(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}

func hasRPCClientTarget(cfg zrpc.RpcClientConf) bool {
	if strings.TrimSpace(cfg.Target) != "" {
		return true
	}
	if len(cfg.Endpoints) > 0 {
		return true
	}
	return len(cfg.Etcd.Hosts) > 0 && strings.TrimSpace(cfg.Etcd.Key) != ""
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
