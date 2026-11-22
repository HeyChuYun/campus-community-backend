package appconfig

import (
	"path/filepath"
	"reflect"
	"testing"

	"hfjj/backend/pkg/configx"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

func TestLoadCaptchaSolverConfig(t *testing.T) {
	configDir := filepath.Join("..", "..", "config")

	options := LoadOptions{
		Config:       configDir,
		Mode:         modeTest,
		SharedConfig: filepath.Join(configDir, "shared.local.yaml"),
	}
	options = normalizeLoadOptions("captcha-solver", options)
	shared, err := loadSharedConfig(options.SharedConfig)
	if err != nil {
		t.Fatalf("loadSharedConfig returned error: %v", err)
	}
	shared = enrichSharedEtcd(shared, options)
	_, serviceBlock, err := loadConfigBlocks("captcha-solver", options)
	if err != nil {
		t.Fatalf("loadConfigBlocks returned error: %v", err)
	}
	merged := buildServiceMap("captcha-solver", shared, serviceBlock)
	applyRuntimeBootstrap("captcha-solver", shared, merged, options)

	t.Logf("serviceBlock=%#v", serviceBlock)
	t.Logf("merged=%#v", merged)

	if _, ok := valueByKeys(merged, "listenOn", "ListenOn"); !ok {
		t.Fatalf("listenOn not found in merged map")
	}
}

type testGatewayConfig struct {
	Redis       gozeroredis.RedisKeyConf
	PassportRpc zrpc.RpcClientConf
	RelationRpc zrpc.RpcClientConf
	Commercial  struct {
		Membership struct {
			Policies []map[string]any `json:",optional"`
		} `json:",optional"`
	} `json:",optional"`
}

type testContentConfig struct {
	Content struct {
		Media struct {
			MaxImageCount int64
		}
		Membership struct {
			Policies []map[string]any `json:",optional"`
		} `json:",optional"`
	}
}

func TestLoadGatewayRedisConfig(t *testing.T) {
	configDir := filepath.Join("..", "..", "config")

	var c testGatewayConfig
	err := LoadService("gateway", LoadOptions{
		Config:       configDir,
		Mode:         modeTest,
		SharedConfig: filepath.Join(configDir, "shared.local.yaml"),
	}, &c)
	if err != nil {
		t.Fatalf("LoadService returned error: %v", err)
	}

	if c.Redis.Host == "" {
		t.Fatalf("gateway redis host is empty: %+v", c.Redis)
	}
	if c.Redis.Key == "" {
		t.Fatalf("gateway redis key is empty: %+v", c.Redis)
	}
	if err := configx.ValidateRPCClient("passport rpc", c.PassportRpc); err != nil {
		t.Fatalf("gateway passport rpc config invalid: %v, cfg=%+v", err, c.PassportRpc)
	}
	if err := configx.ValidateRPCClient("relation rpc", c.RelationRpc); err != nil {
		t.Fatalf("gateway relation rpc config invalid: %v, cfg=%+v", err, c.RelationRpc)
	}
}

func TestLoadContentMembershipConfig(t *testing.T) {
	configDir := filepath.Join("..", "..", "config")

	var c testContentConfig
	err := LoadService("content", LoadOptions{
		Config:       configDir,
		Mode:         modeTest,
		SharedConfig: filepath.Join(configDir, "shared.local.yaml"),
	}, &c)
	if err != nil {
		t.Fatalf("LoadService returned error: %v", err)
	}

	if c.Content.Media.MaxImageCount <= 0 {
		t.Fatalf("content media config not loaded: %+v", c.Content.Media)
	}
}

func TestApplyRPCClientBootstrapFromSharedEtcd(t *testing.T) {
	shared := SharedConfig{
		Etcd: EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379", "127.0.0.1:2379"},
			Namespace: "/hfjj/prod",
		},
	}
	merged := map[string]any{}

	applyRPCClientBootstrap(shared, merged)

	passport, ok := mapValueByKeys(merged, "passportRpc")
	if !ok {
		t.Fatalf("passportRpc not generated")
	}
	etcd, ok := mapValueByKeys(passport, "etcd")
	if !ok {
		t.Fatalf("passportRpc.etcd not generated")
	}
	hosts, ok := valueByKeys(etcd, "hosts")
	if !ok || !hasStringSliceContent(hosts) {
		t.Fatalf("passportRpc.etcd.hosts invalid: %+v", etcd)
	}
	key, ok := valueByKeys(etcd, "key")
	if !ok || key != "/hfjj/prod/rpc/passport" {
		t.Fatalf("passportRpc.etcd.key invalid: %+v", etcd)
	}
}

func TestBuildServiceMapMergesServiceConfigOverShared(t *testing.T) {
	shared := SharedConfig{
		Redis: map[string]any{
			"host": "redis:6379",
			"type": "node",
			"pass": "shared-password",
		},
	}
	serviceBlock := map[string]any{
		"appRedis": map[string]any{
			"key":  "relation:",
			"pass": "service-password",
		},
	}

	merged := buildServiceMap("relation", shared, serviceBlock)

	appRedis, ok := mapValueByKeys(merged, "appRedis")
	if !ok {
		t.Fatalf("appRedis not generated")
	}
	if appRedis["host"] != "redis:6379" {
		t.Fatalf("shared appRedis.host should be preserved, got=%v", appRedis["host"])
	}
	if appRedis["type"] != "node" {
		t.Fatalf("shared appRedis.type should be preserved, got=%v", appRedis["type"])
	}
	if appRedis["key"] != "relation:" {
		t.Fatalf("service appRedis.key should be applied, got=%v", appRedis["key"])
	}
	if appRedis["pass"] != "service-password" {
		t.Fatalf("service appRedis.pass should override shared value, got=%v", appRedis["pass"])
	}
}

func TestEnrichSharedEtcdPrefersEnvOverShared(t *testing.T) {
	t.Setenv("ETCD_ENDPOINTS", "103.238.187.226:2379,103.238.187.227:2379")
	t.Setenv("CONFIG_NAMESPACE", "/hfjj")

	shared := SharedConfig{
		Etcd: EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
			Namespace: "/hfjj/local",
		},
	}

	shared = enrichSharedEtcd(shared, LoadOptions{Mode: modeProd})

	wantEndpoints := []string{"103.238.187.226:2379", "103.238.187.227:2379"}
	if !reflect.DeepEqual(shared.Etcd.Endpoints, wantEndpoints) {
		t.Fatalf("etcd endpoints should prefer env, got=%v want=%v", shared.Etcd.Endpoints, wantEndpoints)
	}
	if shared.Etcd.Namespace != "/hfjj/prod" {
		t.Fatalf("etcd namespace should prefer env, got=%q", shared.Etcd.Namespace)
	}
}

func TestEnrichSharedEtcdFallsBackToSharedFile(t *testing.T) {
	shared := SharedConfig{
		Etcd: EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
			Namespace: "/hfjj/local",
		},
	}

	shared = enrichSharedEtcd(shared, LoadOptions{Mode: modeProd})

	wantEndpoints := []string{"127.0.0.1:2379"}
	if !reflect.DeepEqual(shared.Etcd.Endpoints, wantEndpoints) {
		t.Fatalf("etcd endpoints should fall back to shared config, got=%v want=%v", shared.Etcd.Endpoints, wantEndpoints)
	}
	if shared.Etcd.Namespace != "/hfjj/local" {
		t.Fatalf("etcd namespace should fall back to shared config, got=%q", shared.Etcd.Namespace)
	}
}

