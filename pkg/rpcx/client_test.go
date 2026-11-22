package rpcx

import (
	"testing"

	"github.com/zeromicro/go-zero/zrpc"
)

func TestNormalizeClientConfUsesCompatTargetForEtcd(t *testing.T) {
	cfg := zrpc.RpcClientConf{}
	cfg.Etcd.Hosts = []string{"127.0.0.1:2379"}
	cfg.Etcd.Key = "/hfjj/prod/rpc/passport"

	got := normalizeClientConf(cfg)
	want := "hfetcd://127.0.0.1:2379/hfjj/prod/rpc/passport"
	if got.Target != want {
		t.Fatalf("target = %q, want %q", got.Target, want)
	}
}

func TestNormalizeClientConfKeepsExplicitTarget(t *testing.T) {
	cfg := zrpc.RpcClientConf{
		Target: "direct:///127.0.0.1:20000",
	}
	cfg.Etcd.Hosts = []string{"127.0.0.1:2379"}
	cfg.Etcd.Key = "/hfjj/prod/rpc/passport"

	got := normalizeClientConf(cfg)
	if got.Target != cfg.Target {
		t.Fatalf("target = %q, want keep explicit target %q", got.Target, cfg.Target)
	}
}
