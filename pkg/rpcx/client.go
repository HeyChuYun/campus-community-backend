package rpcx

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/resolver"
)

const compatEtcdScheme = "hfetcd"

func init() {
	// 注册项目自己的 etcd 兼容 resolver。
	// 原因是当前 go-zero 内置 resolver 在本项目版本组合下会触发 bad resolver state。
	resolver.Register(&compatEtcdBuilder{})
}

// MustNewClient 创建 RPC 客户端，优先复用 go-zero 配置结构，
// 仅在 etcd 发现模式下切换到项目兼容 resolver，保留服务发现能力。
func MustNewClient(c zrpc.RpcClientConf, options ...zrpc.ClientOption) zrpc.Client {
	cli, err := NewClient(c, options...)
	logx.Must(err)
	return cli
}

// NewClient 创建 RPC 客户端。
func NewClient(c zrpc.RpcClientConf, options ...zrpc.ClientOption) (zrpc.Client, error) {
	return zrpc.NewClient(normalizeClientConf(c), options...)
}

func normalizeClientConf(c zrpc.RpcClientConf) zrpc.RpcClientConf {
	// 只有在未显式指定 Target/Endpoints 且启用了 etcd 发现时，才切换为兼容 target。
	if len(c.Endpoints) == 0 && strings.TrimSpace(c.Target) == "" &&
		len(c.Etcd.Hosts) > 0 && strings.TrimSpace(c.Etcd.Key) != "" {
		c.Target = buildCompatEtcdTarget(c.Etcd.Hosts, c.Etcd.Key)
	}

	return c
}

func buildCompatEtcdTarget(hosts []string, key string) string {
	return fmt.Sprintf("%s://%s/%s", compatEtcdScheme, strings.Join(hosts, ","), strings.TrimPrefix(key, "/"))
}

type compatEtcdBuilder struct{}

func (b *compatEtcdBuilder) Scheme() string {
	return compatEtcdScheme
}

func (b *compatEtcdBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	hosts := strings.Split(target.URL.Host, ",")
	key := "/" + strings.TrimPrefix(target.URL.Path, "/")

	sub, err := discov.NewSubscriber(hosts, key)
	if err != nil {
		return nil, err
	}

	update := func() {
		values := sub.Values()
		addrs := make([]resolver.Address, 0, len(values))
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			addrs = append(addrs, resolver.Address{Addr: value})
		}

		// 这里故意使用 NewAddress，而不是 UpdateState。
		// 本项目线上实际验证表明，NewAddress 可以稳定把 etcd 地址推送给 gRPC balancer。
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

// WaitReady 用于诊断或少量初始化场景，普通业务代码通常不需要主动调用。
func WaitReady(ctx context.Context, client zrpc.Client) bool {
	conn := client.Conn()
	state := conn.GetState()
	for {
		if state.String() == "READY" {
			return true
		}
		if !conn.WaitForStateChange(ctx, state) {
			return conn.GetState().String() == "READY"
		}
		state = conn.GetState()
	}
}
