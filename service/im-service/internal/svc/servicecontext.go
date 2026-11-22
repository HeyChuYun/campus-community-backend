package svc

import (
	"strings"

	"hfjj/backend/im-service/internal/config"
	"hfjj/backend/im-service/internal/mqconsumer"
	"hfjj/backend/im-service/internal/repository"
	"hfjj/backend/im-service/internal/wukong"
	"hfjj/backend/pkg/rpcx"
	"hfjj/backend/proto/relationpb"

	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config              config.Config
	GormDB              *gorm.DB
	Repo                *repository.Repository
	RelationRPC         relationpb.RelationServiceClient
	WuKongClient        *wukong.Client
	InteractionConsumer *mqconsumer.Consumer
}

func NewServiceContext(c config.Config) *ServiceContext {
	gormDB := newGormPostgresClient(c.Postgres.DSN)
	repo := repository.NewRepository(gormDB)
	wkClient := wukong.NewClient(c.WuKong)

	var relationClient relationpb.RelationServiceClient
	if hasRPCClientTarget(c.RelationRpc) {
		// IM 侧关系服务调用也切到兼容 resolver，避免生产环境解析到空地址。
		relationClient = relationpb.NewRelationServiceClient(rpcx.MustNewClient(c.RelationRpc).Conn())
	}

	ctx := &ServiceContext{
		Config:       c,
		GormDB:       gormDB,
		Repo:         repo,
		RelationRPC:  relationClient,
		WuKongClient: wkClient,
	}

	if c.Consumer.Enabled {
		ctx.InteractionConsumer = mqconsumer.NewConsumer(c, repo, relationClient, wkClient)
	}
	return ctx
}

func newGormPostgresClient(dsn string) *gorm.DB {
	if strings.TrimSpace(dsn) == "" {
		return nil
	}
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
