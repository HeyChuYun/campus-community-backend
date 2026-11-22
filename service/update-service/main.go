package main

import (
	"flag"
	"fmt"

	"hfjj/backend/pkg/appconfig"
	"hfjj/backend/proto/updatepb"
	"hfjj/backend/update-service/internal/config"
	"hfjj/backend/update-service/internal/server"
	"hfjj/backend/update-service/internal/svc"

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
	appconfig.MustLoadService("update", buildLoadOptions(*configFile, *configFileCompat, *mode, *sharedConfig, *etcdEndpoints, *configNamespace, *etcdNamespace), &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		updatepb.RegisterUpdateServiceServer(grpcServer, server.NewUpdateServiceServer(ctx))

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
