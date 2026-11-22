package main

import (
	"flag"
	"fmt"

	"hfjj/backend/gateway/internal/config"
	"hfjj/backend/gateway/internal/handler"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/pkg/appconfig"

	"github.com/zeromicro/go-zero/rest"
)

// `-f` 统一接收配置目录或服务配置文件路径，默认读取 backend/config。
var configFile = flag.String("config", "config", "service config file or config directory")
var mode = flag.String("mode", "prod", "config mode: prod or test")
var sharedConfig = flag.String("sharedConfig", "", "shared config file path")
var etcdEndpoints = flag.String("etcdEndpoints", "", "etcd endpoints, comma-separated")
var configNamespace = flag.String("configNamespace", "", "config root namespace, e.g. /hfjj")
var etcdNamespace = flag.String("etcdNamespace", "", "full etcd namespace, e.g. /hfjj/prod")

func main() {
	// 将 命令中的参数 写入变量值
	flag.Parse()

	// 定义配置变量
	var c config.Config

	// 启动服务
	appconfig.MustLoadService("gateway", appconfig.LoadOptions{
		Config:          *configFile,
		Mode:            *mode,
		SharedConfig:    *sharedConfig,
		EtcdEndpoints:   *etcdEndpoints,
		ConfigNamespace: *configNamespace,
		EtcdNamespace:   *etcdNamespace,
	}, &c)
	ctx := svc.NewServiceContext(c)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting gateway at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
