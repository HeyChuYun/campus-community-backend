package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/config"
	"hfjj/backend/pkg/appconfig"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/resolver"
)

var configFile = flag.String("config", "config", "service config file or config directory")
var configFileCompat = flag.String("f", "", "compat alias for -config")
var mode = flag.String("mode", "prod", "config mode: prod or test")
var sharedConfig = flag.String("sharedConfig", "", "shared config file path")
var etcdEndpoints = flag.String("etcdEndpoints", "", "etcd endpoints, comma-separated")
var configNamespace = flag.String("configNamespace", "", "config root namespace, e.g. /hfjj")
var etcdNamespace = flag.String("etcdNamespace", "", "full etcd namespace, e.g. /hfjj/prod")
var noProxy = flag.Bool("no-proxy", false, "disable grpc proxy resolver for diagnosis")
var useNewClient = flag.Bool("new-client", false, "use grpc.NewClient instead of zrpc.NewClient for diagnosis")
var useCompatResolver = flag.Bool("compat-resolver", false, "use local etcd resolver with cc.NewAddress for diagnosis")

const compatScheme = "hfetcdtest"

func main() {
	flag.Parse()

	var c config.Config
	appconfig.MustLoadService("gateway", buildLoadOptions(*configFile, *configFileCompat, *mode, *sharedConfig, *etcdEndpoints, *configNamespace, *etcdNamespace), &c)

	if err := diagnose("passport", c.PassportRpc); err != nil {
		panic(err)
	}
}

func diagnose(name string, conf zrpc.RpcClientConf) error {
	fmt.Println("===", name, "===")
	target, err := conf.BuildTarget()
	if err != nil {
		return fmt.Errorf("build target failed: %w", err)
	}
	fmt.Println("target:", target)
	fmt.Printf("etcd hosts=%v key=%s nonblock=%v balancer=%s timeout=%d\n",
		conf.Etcd.Hosts, conf.Etcd.Key, conf.NonBlock, conf.BalancerName, conf.Timeout)

	if len(conf.Etcd.Hosts) > 0 && len(conf.Etcd.Key) > 0 {
		sub, err := discov.NewSubscriber(conf.Etcd.Hosts, conf.Etcd.Key)
		if err != nil {
			return fmt.Errorf("subscriber create failed: %w", err)
		}
		defer sub.Close()
		fmt.Println("subscriber values:", sub.Values())
	}

	var conn *grpc.ClientConn
	if *useCompatResolver {
		target = buildCompatTarget(conf.Etcd.Hosts, conf.Etcd.Key)
		fmt.Println("compat target:", target)
		dialOpts := buildRawDialOptions(conf)
		rawConn, err := grpc.NewClient(target, dialOpts...)
		if err != nil {
			return fmt.Errorf("grpc.NewClient with compat resolver failed: %w", err)
		}
		conn = rawConn
		conn.Connect()
	} else if *useNewClient {
		rawConn, err := grpc.NewClient(target, buildRawDialOptions(conf)...)
		if err != nil {
			return fmt.Errorf("grpc.NewClient failed: %w", err)
		}
		conn = rawConn
		conn.Connect()
	} else {
		var opts []zrpc.ClientOption
		if *noProxy {
			// 用于验证 gRPC 代理解析层是否导致 etcd 发现结果被判定为 bad resolver state。
			opts = append(opts, zrpc.WithDialOption(grpc.WithNoProxy()))
		}

		cli, err := zrpc.NewClient(conf, opts...)
		if err != nil {
			return fmt.Errorf("new client failed: %w", err)
		}
		conn = cli.Conn()
	}
	fmt.Println("initial state:", conn.GetState())

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	lastState := conn.GetState()
	for {
		if !conn.WaitForStateChange(ctx, lastState) {
			break
		}
		lastState = conn.GetState()
		fmt.Println("state changed to:", lastState)
	}
	fmt.Println("final state:", conn.GetState())

	healthClient := healthpb.NewHealthClient(conn)
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer healthCancel()

	resp, err := healthClient.Check(healthCtx, &healthpb.HealthCheckRequest{})
	if err != nil {
		fmt.Println("health check error:", err)
	} else {
		fmt.Println("health status:", resp.GetStatus().String())
	}

	if conn.GetState() == connectivity.TransientFailure {
		fmt.Println("connection is in transient failure")
	}

	return nil
}

func buildRawDialOptions(conf zrpc.RpcClientConf) []grpc.DialOption {
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, conf.BalancerName)),
	}
	if *noProxy {
		dialOpts = append(dialOpts, grpc.WithNoProxy())
	}
	return dialOpts
}

func buildCompatTarget(hosts []string, key string) string {
	return fmt.Sprintf("%s://%s/%s", compatScheme, strings.Join(hosts, ","), strings.TrimPrefix(key, "/"))
}

func init() {
	resolver.Register(&compatEtcdBuilder{})
}

type compatEtcdBuilder struct{}

func (b *compatEtcdBuilder) Scheme() string {
	return compatScheme
}

func (b *compatEtcdBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	hosts := strings.Split(target.URL.Host, ",")
	key := "/" + strings.TrimPrefix(target.URL.Path, "/")
	sub, err := discov.NewSubscriber(hosts, key)
	if err != nil {
		return nil, err
	}

	update := func() {
		vals := sub.Values()
		addrs := make([]resolver.Address, 0, len(vals))
		for _, val := range vals {
			addrs = append(addrs, resolver.Address{Addr: val})
		}
		cc.NewAddress(addrs)
	}

	sub.AddListener(update)
	update()
	return &compatEtcdResolver{sub: sub}, nil
}

type compatEtcdResolver struct {
	sub *discov.Subscriber
}

func (r *compatEtcdResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *compatEtcdResolver) Close() {
	r.sub.Close()
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
