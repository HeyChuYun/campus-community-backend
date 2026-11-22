package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"hfjj/backend/content-service/internal/config"
	"hfjj/backend/content-service/internal/logic"
	"hfjj/backend/content-service/internal/mqconsumer"
	"hfjj/backend/content-service/internal/server"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/pkg/appconfig"
	"hfjj/backend/proto/contentpb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// `-f` 统一接收配置目录或服务配置文件路径，默认读取 backend/config。
var configFile = flag.String("config", "config", "service config file or config directory")
var configFileCompat = flag.String("f", "", "compat alias for -config")
var mode = flag.String("mode", "prod", "config mode: prod or test")
var sharedConfig = flag.String("sharedConfig", "", "shared config file path")
var etcdEndpoints = flag.String("etcdEndpoints", "", "etcd endpoints, comma-separated")
var configNamespace = flag.String("configNamespace", "", "config root namespace, e.g. /hfjj")
var etcdNamespace = flag.String("etcdNamespace", "", "full etcd namespace, e.g. /hfjj/prod")

func main() {
	flag.Parse()

	var c config.Config
	appconfig.MustLoadService("content", buildLoadOptions(*configFile, *configFileCompat, *mode, *sharedConfig, *etcdEndpoints, *configNamespace, *etcdNamespace), &c)
	ctx := svc.NewServiceContext(c)
	logx.Infof(
		"content-service boot config: moderationEnabled=%t moderationQueueEnabled=%t rabbitmqEnabled=%t outboxEnabled=%t moderationProvider=%s",
		c.Moderation.Enabled,
		c.Moderation.ReviewQueue.Enabled,
		c.RabbitMQ.Enabled,
		c.Outbox.Enabled,
		c.Moderation.Provider,
	)

	if ctx.Dispatcher != nil {
		ctx.Dispatcher.Start()
		defer ctx.Dispatcher.Stop()
	}

	var moderationConsumer *mqconsumer.Consumer
	if c.Moderation.Enabled && c.Moderation.ReviewQueue.Enabled {
		moderationConsumer = mqconsumer.NewConsumer(ctx)
		if err := moderationConsumer.Start(); err != nil {
			panic(err)
		}
		defer moderationConsumer.Stop()
	}

	if c.Moderation.Enabled {
		stopPolling := make(chan struct{})
		defer close(stopPolling)
		go func() {
			ticker := time.NewTicker(c.Moderation.RetryInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = logic.PollPendingModerationTasks(context.Background(), ctx)
				case <-stopPolling:
					return
				}
			}
		}()
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		contentpb.RegisterContentServiceServer(grpcServer, server.NewContentServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

func buildLoadOptions(configValue string, compatValue string, modeValue string, sharedConfigValue string, etcdEndpointsValue string, configNamespaceValue string, etcdNamespaceValue string) appconfig.LoadOptions {
	if compatValue != "" {
		configValue = compatValue
	}
	return appconfig.LoadOptions{
		Config:          configValue,
		Mode:            modeValue,
		SharedConfig:    sharedConfigValue,
		EtcdEndpoints:   etcdEndpointsValue,
		ConfigNamespace: configNamespaceValue,
		EtcdNamespace:   etcdNamespaceValue,
	}
}
