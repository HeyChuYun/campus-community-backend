package appconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/pkg/configx"

	"github.com/zeromicro/go-zero/core/conf"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

const (
	// 统一定义支持的配置来源，避免代码里散落魔法字符串。
	modeProd         = "prod"
	configSourceFile = "file"
	configSourceEtcd = "etcd"
	modeTest         = "test"
)

type LoadOptions struct {
	Config          string
	Mode            string
	SharedConfig    string
	EtcdEndpoints   string
	ConfigNamespace string
	EtcdNamespace   string
}

// SharedConfig 表示所有服务共享的基础设施配置。
// 这部分配置既会在本地文件模式下参与合并，也会在 etcd 模式下参与合并。
type SharedConfig struct {
	Etcd     EtcdConfig `yaml:"etcd"`
	Postgres any        `yaml:"postgres"`
	Redis    any        `yaml:"redis"`
	MQ       any        `yaml:"mq"`
}

// EtcdConfig Etcd配置
type EtcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
	Namespace string   `yaml:"namespace"`
}

func MustLoadService(serviceName string, options LoadOptions, v any) {
	// 加载服务
	if err := LoadService(serviceName, options, v); err != nil {
		// 抛错
		panic(err)
	}
}

// LoadService 会根据运行环境自动选择 file 或 etcd 作为配置来源，
// 最终为单个服务组装出一份完整的 go-zero 配置结构。
func LoadService(serviceName string, options LoadOptions, v any) error {
	options = normalizeLoadOptions(serviceName, options)

	// 加载配置
	shared, serviceBlock, err := loadConfigBlocks(serviceName, options)
	if err != nil {
		return err
	}

	// 先灌入共享基础设施配置，再覆盖当前服务自己的业务配置。
	merged := map[string]any{}
	merged = buildServiceMap(serviceName, shared, serviceBlock)
	fillSharedServiceFields(serviceName, shared, merged)

	// 最后根据运行环境补齐 go-zero 启动必需字段。
	applyRuntimeBootstrap(serviceName, shared, merged, options)
	// 统一补齐各服务的 RPC client 配置，生产环境可直接复用 shared.etcd。
	applyRPCClientBootstrap(shared, merged)

	// Yaml序列化
	data, err := yaml.Marshal(merged)
	if err != nil {
		return err
	}
	if err := conf.LoadFromYamlBytes(data, v); err != nil {
		return err
	}
	if err := normalizeHostsFields(v); err != nil {
		return err
	}
	return configx.Validate(v)
}

func normalizeLoadOptions(serviceName string, options LoadOptions) LoadOptions {
	options.Mode = normalizeMode(options.Mode)
	if options.Mode == modeTest {
		if strings.TrimSpace(options.SharedConfig) == "" {
			options.SharedConfig = resolveSharedConfigPath(options.Config)
		}
		if strings.TrimSpace(options.Config) == "" {
			options.Config = "config"
		}
		options.Config = resolveServiceConfigPath(serviceName, options.Config)
	}
	return options
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "prod", "production":
		return modeProd
	case "test", "testing", "local", "dev", "development", "debug":
		return modeTest
	default:
		return modeProd
	}
}

func loadSharedConfig(sharedPath string) (SharedConfig, error) {
	sharedData, err := os.ReadFile(sharedPath)
	if err != nil {
		return SharedConfig{}, fmt.Errorf("read shared config failed: %w", err)
	}
	shared, err := parseSharedConfig(sharedData)
	if err != nil {
		return SharedConfig{}, fmt.Errorf("parse shared config failed: %w", err)
	}
	return shared, nil
}

func loadConfigBlocks(serviceName string, options LoadOptions) (SharedConfig, map[string]any, error) {
	if options.Mode == modeProd {
		return loadConfigBlocksFromEtcd(serviceName, options)
	}
	return loadConfigBlocksFromFile(options)
}

func loadConfigBlocksFromFile(options LoadOptions) (SharedConfig, map[string]any, error) {
	shared, err := loadSharedConfig(options.SharedConfig)
	if err != nil {
		return SharedConfig{}, nil, err
	}
	shared = enrichSharedEtcd(shared, options)

	serviceBlock, err := loadServiceBlockFromFile(options.Config)
	if err != nil {
		return SharedConfig{}, nil, err
	}
	return shared, serviceBlock, nil
}

func loadServiceBlockFromFile(servicePath string) (map[string]any, error) {
	serviceData, err := os.ReadFile(servicePath)
	if err != nil {
		return nil, fmt.Errorf("read service config failed: %w", err)
	}
	serviceBlock, err := parseMap(serviceData)
	if err != nil {
		return nil, fmt.Errorf("parse service config failed: %w", err)
	}
	return serviceBlock, nil
}

func loadConfigBlocksFromEtcd(serviceName string, options LoadOptions) (SharedConfig, map[string]any, error) {
	cli, namespace, err := newEtcdClient(options)
	if err != nil {
		return SharedConfig{}, nil, err
	}
	defer cli.Close()

	sharedData, err := getEtcdValue(cli, resolveSharedEtcdKey(namespace))
	if err != nil {
		return SharedConfig{}, nil, fmt.Errorf("load shared config from etcd failed: %w", err)
	}
	shared, err := parseSharedConfig(sharedData)
	if err != nil {
		return SharedConfig{}, nil, fmt.Errorf("parse shared etcd config failed: %w", err)
	}
	shared = enrichSharedEtcd(shared, options)

	serviceKey := resolveServiceEtcdKey(serviceName, namespace)
	serviceData, err := getEtcdValue(cli, serviceKey)
	if err != nil {
		return SharedConfig{}, nil, fmt.Errorf("load service config from etcd failed: %w", err)
	}
	serviceBlock, err := parseMap(serviceData)
	if err != nil {
		return SharedConfig{}, nil, fmt.Errorf("parse service etcd config failed: %w", err)
	}
	return shared, serviceBlock, nil
}

func newEtcdClient(options LoadOptions) (*clientv3.Client, string, error) {
	settings := resolveEtcdSettings(options)
	if len(settings.Endpoints) == 0 {
		return nil, "", errors.New("ETCD_ENDPOINTS is empty")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   settings.Endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, "", err
	}
	return cli, settings.Namespace, nil
}

func getEtcdValue(cli *clientv3.Client, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("etcd config key not found: %s", key)
	}
	return resp.Kvs[0].Value, nil
}

func parseSharedConfig(data []byte) (SharedConfig, error) {
	var shared SharedConfig
	if err := yaml.Unmarshal(data, &shared); err != nil {
		return SharedConfig{}, err
	}
	return shared, nil
}

func parseMap(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func resolveSharedConfigPath(configPath string) string {
	cleanPath := filepath.Clean(strings.TrimSpace(configPath))
	if cleanPath == "." || cleanPath == "" {
		cleanPath = "config"
	}
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		for _, candidate := range []string{
			filepath.Join(cleanPath, "shared.yml"),
			filepath.Join(cleanPath, "shared.yaml"),
			filepath.Join(cleanPath, "shared.local.yaml"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return filepath.Join(cleanPath, "shared.yml")
	}
	base := strings.ToLower(filepath.Base(cleanPath))
	if base == "shared.yml" || base == "shared.yaml" || base == "shared.local.yaml" {
		return cleanPath
	}
	return resolveSharedConfigPath(filepath.Dir(cleanPath))
}

func resolveServiceConfigPath(serviceName string, configPath string) string {
	cleanPath := filepath.Clean(strings.TrimSpace(configPath))
	if cleanPath == "." || cleanPath == "" {
		cleanPath = "config"
	}
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		for _, candidate := range []string{
			filepath.Join(cleanPath, "services", serviceName+".yml"),
			filepath.Join(cleanPath, "services", serviceName+".yaml"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return filepath.Join(cleanPath, "services", serviceName+".yml")
	}
	if strings.EqualFold(filepath.Ext(cleanPath), ".yml") || strings.EqualFold(filepath.Ext(cleanPath), ".yaml") {
		return cleanPath
	}
	return filepath.Join(cleanPath, "services", serviceName+".yml")
}

func buildServiceMap(service string, shared SharedConfig, serviceBlock map[string]any) map[string]any {
	merged := shared.toMap(service)
	mergeMap(merged, serviceBlock)
	return merged
}

func mergeMap(base map[string]any, override map[string]any) {
	for key, value := range override {
		baseMap, baseOK := base[key].(map[string]any)
		overrideMap, overrideOK := value.(map[string]any)
		if baseOK && overrideOK {
			mergeMap(baseMap, overrideMap)
			continue
		}
		base[key] = value
	}
}

func fillSharedServiceFields(service string, shared SharedConfig, merged map[string]any) {
	if service == "gateway" {
		fillRedisFromShared(shared, merged)
	}
}

// resolveFilePaths 支持三种传参方式：
// 1. 传配置目录，例如 config
// 2. 传具体服务配置，例如 config/services/content.yaml
// 3. 兼容历史传参 config/local.yaml，此时会回退为目录模式
func resolveFilePaths(serviceName string, filePath string) (string, string) {
	if strings.TrimSpace(filePath) == "" {
		filePath = firstNonEmptyEnv("CONFIG_DIR", "config")
	}

	cleanPath := filepath.Clean(filePath)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return filepath.Join(cleanPath, "shared.local.yaml"), filepath.Join(cleanPath, "services", serviceName+".yaml")
	}

	if strings.EqualFold(filepath.Base(cleanPath), "local.yaml") {
		configDir := filepath.Dir(cleanPath)
		return filepath.Join(configDir, "shared.local.yaml"), filepath.Join(configDir, "services", serviceName+".yaml")
	}

	if strings.EqualFold(filepath.Base(cleanPath), "shared.local.yaml") {
		configDir := filepath.Dir(cleanPath)
		return cleanPath, filepath.Join(configDir, "services", serviceName+".yaml")
	}

	if strings.EqualFold(filepath.Base(filepath.Dir(cleanPath)), "services") {
		configDir := filepath.Dir(filepath.Dir(cleanPath))
		return filepath.Join(configDir, "shared.local.yaml"), cleanPath
	}

	// 当路径不存在时，如果看起来像目录，则按目录模式处理；
	// 否则按“文件所在目录”回退，保证手工传参时仍然可预测。
	if filepath.Ext(cleanPath) == "" {
		return filepath.Join(cleanPath, "shared.local.yaml"), filepath.Join(cleanPath, "services", serviceName+".yaml")
	}
	configDir := filepath.Dir(cleanPath)
	return filepath.Join(configDir, "shared.local.yaml"), filepath.Join(configDir, "services", serviceName+".yaml")
}

func (s SharedConfig) toMap(service string) map[string]any {
	result := map[string]any{
		"sharedEtcd": s.Etcd,
		"postgres":   s.Postgres,
		"rabbitMQ":   s.MQ,
	}

	// 注意：
	// 1. gateway 自己显式定义了顶层 Redis 字段，需要合并共享 host/type/pass 与服务侧 key。
	// 2. RPC 服务嵌入的 zrpc.RpcServerConf 也自带一个顶层 Redis 字段，
	//    那个字段用于 go-zero 的 RPC 鉴权限流，要求必须带 key。
	//    如果把共享 Redis 直接灌进去，会在没有 key 时触发 `field "redis.Key" is not set`。
	// 因此这里不能把共享 Redis 无差别注入到所有服务的顶层 Redis。
	if service == "gateway" {
		result["redis"] = s.Redis
	}

	// 业务 Redis 统一通过 AppRedis 传递给各 RPC 服务。
	result["appRedis"] = s.Redis
	return result
}

func fillRedisFromShared(shared SharedConfig, merged map[string]any) {
	sharedRedis, ok := shared.Redis.(map[string]any)
	if !ok {
		return
	}
	existing, ok := mapValueByKeys(merged, "redis", "Redis")
	if !ok {
		merged["redis"] = sharedRedis
		return
	}
	for _, key := range []string{"host", "Host", "type", "Type", "pass", "Pass", "tls", "Tls"} {
		if _, exists := existing[key]; exists {
			continue
		}
		if value, exists := sharedRedis[key]; exists {
			existing[key] = value
		}
	}
}

func applyRuntimeBootstrap(service string, shared SharedConfig, merged map[string]any, options LoadOptions) {
	if service == "" {
		service = os.Getenv("SERVICE_NAME")
	}

	shared = enrichSharedEtcd(shared, options)

	if !hasAnyKey(merged, "mode", "Mode") {
		merged["mode"] = runtimeMode(options.Mode)
	}
	if !hasAnyKey(merged, "telemetry", "Telemetry") {
		merged["telemetry"] = map[string]any{}
	}
	if telemetry, ok := mapValueByKeys(merged, "telemetry", "Telemetry"); ok {
		if _, exists := telemetry["name"]; !exists {
			if _, exists = telemetry["Name"]; !exists {
				telemetry["name"] = runtimeServiceName(service)
			}
		}
	}

	if service == "gateway" {
		applyRestBootstrap(service, merged)
		return
	}
	applyRPCBootstrap(service, shared, merged)
}

// enrichSharedEtcd 保证共享配置里至少有一份可用的注册中心信息，
// 这样本地文件模式和 etcd 模式都能正确补齐 go-zero 的 Etcd 字段。
func enrichSharedEtcd(shared SharedConfig, options LoadOptions) SharedConfig {
	settings := resolveEtcdSettings(options)
	if len(settings.Endpoints) > 0 {
		shared.Etcd.Endpoints = settings.Endpoints
	} else if len(shared.Etcd.Endpoints) == 0 {
		shared.Etcd.Endpoints = nil
	}
	if settings.Namespace != "" {
		shared.Etcd.Namespace = settings.Namespace
	} else if strings.TrimSpace(shared.Etcd.Namespace) == "" {
		shared.Etcd.Namespace = resolveDiscoveryNamespace(options)
	}
	return shared
}

func resolveEtcdSettings(options LoadOptions) EtcdConfig {
	return EtcdConfig{
		Endpoints: splitCSV(firstNonEmpty(options.EtcdEndpoints, firstNonEmptyEnv("ETCD_ENDPOINTS"))),
		Namespace: explicitEtcdNamespace(options),
	}
}

func explicitEtcdNamespace(options LoadOptions) string {
	if legacy := strings.TrimSpace(firstNonEmpty(options.EtcdNamespace, firstNonEmptyEnv("ETCD_NAMESPACE"))); legacy != "" {
		return strings.TrimRight(legacy, "/")
	}
	namespaceRoot := strings.TrimSpace(firstNonEmpty(options.ConfigNamespace, firstNonEmptyEnv("CONFIG_NAMESPACE")))
	if namespaceRoot != "" {
		return strings.TrimRight(namespaceRoot, "/") + "/" + normalizeMode(options.Mode)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func applyRestBootstrap(service string, merged map[string]any) {
	if !hasAnyKey(merged, "name", "Name") {
		merged["name"] = runtimeServiceName(service)
	}
	if !hasAnyKey(merged, "host", "Host") {
		merged["host"] = envOrDefault("0.0.0.0", "SERVICE_HOST", "HOST")
	}
	if !hasAnyKey(merged, "port", "Port") {
		merged["port"] = intFromEnv(portEnvKey(service), defaultPort(service))
	}
	if !hasAnyKey(merged, "timeout", "Timeout") {
		merged["timeout"] = intFromEnv("SERVICE_TIMEOUT", 10000)
	}
	if !hasAnyKey(merged, "maxBytes", "MaxBytes") {
		merged["maxBytes"] = intFromEnv("SERVICE_MAX_BYTES", 1048576)
	}
}

func applyRPCBootstrap(service string, shared SharedConfig, merged map[string]any) {
	if name, ok := valueByKeys(merged, "name", "Name"); ok {
		merged["name"] = toString(name)
	} else {
		merged["name"] = service + ".rpc"
	}
	if listenOn, ok := valueByKeys(merged, "listenOn", "ListenOn"); ok {
		merged["listenOn"] = envOrDefault(toString(listenOn), "SERVICE_LISTEN", "LISTEN_ON")
	} else {
		merged["listenOn"] = envOrDefault("0.0.0.0:"+strconv.Itoa(defaultPort(service)), "SERVICE_LISTEN", "LISTEN_ON")
	}
	if !hasAnyKey(merged, "etcd", "Etcd") {
		merged["etcd"] = map[string]any{
			"Hosts": shared.Etcd.Endpoints,
			"Key":   strings.TrimRight(shared.Etcd.Namespace, "/") + "/rpc/" + rpcKey(service),
		}
	}
}

func applyRPCClientBootstrap(shared SharedConfig, merged map[string]any) {
	ensureRPCClientConfig(merged, "updateRpc", "update", shared)
	ensureRPCClientConfig(merged, "passportRpc", "passport", shared)
	ensureRPCClientConfig(merged, "schoolRpc", "school", shared)
	ensureRPCClientConfig(merged, "captchaRpc", "captcha-solver", shared)
	ensureRPCClientConfig(merged, "relationRpc", "relation", shared)
	ensureRPCClientConfig(merged, "contentRpc", "content", shared)
	ensureRPCClientConfig(merged, "imRpc", "im", shared)
}

func ensureRPCClientConfig(merged map[string]any, fieldKey string, targetService string, shared SharedConfig) {
	existing, ok := mapValueByKeys(merged, fieldKey)
	if !ok {
		merged[fieldKey] = defaultRPCClientEtcdConfig(targetService, shared)
		return
	}
	if hasRPCClientTarget(existing) {
		return
	}
	fillRPCClientEtcdFromShared(existing, targetService, shared)
	merged[fieldKey] = existing
}

func fillRPCClientEtcdFromShared(existing map[string]any, targetService string, shared SharedConfig) {
	etcd, ok := mapValueByKeys(existing, "etcd", "Etcd")
	if !ok {
		existing["etcd"] = defaultRPCClientEtcdConfig(targetService, shared)["etcd"]
		return
	}
	if hosts, ok := valueByKeys(etcd, "hosts", "Hosts"); !ok || !hasStringSliceContent(hosts) {
		etcd["hosts"] = append([]string(nil), shared.Etcd.Endpoints...)
	}
	if key, ok := valueByKeys(etcd, "key", "Key"); !ok || strings.TrimSpace(toString(key)) == "" {
		etcd["key"] = strings.TrimRight(shared.Etcd.Namespace, "/") + "/rpc/" + rpcKey(targetService)
	}
}

func hasRPCClientTarget(cfg map[string]any) bool {
	if target, ok := cfg["target"]; ok && strings.TrimSpace(toString(target)) != "" {
		return true
	}
	if target, ok := cfg["Target"]; ok && strings.TrimSpace(toString(target)) != "" {
		return true
	}
	if endpoints, ok := cfg["endpoints"]; ok && hasStringSliceContent(endpoints) {
		return true
	}
	if endpoints, ok := cfg["Endpoints"]; ok && hasStringSliceContent(endpoints) {
		return true
	}
	if etcd, ok := mapValueByKeys(cfg, "etcd", "Etcd"); ok {
		if hosts, ok := valueByKeys(etcd, "hosts", "Hosts"); ok && hasStringSliceContent(hosts) {
			if key, ok := valueByKeys(etcd, "key", "Key"); ok && strings.TrimSpace(toString(key)) != "" {
				return true
			}
		}
	}
	return false
}

func defaultRPCClientEtcdConfig(targetService string, shared SharedConfig) map[string]any {
	return map[string]any{
		"etcd": map[string]any{
			"hosts": append([]string(nil), shared.Etcd.Endpoints...),
			"key":   strings.TrimRight(shared.Etcd.Namespace, "/") + "/rpc/" + rpcKey(targetService),
		},
	}
}

func hasAnyKey(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func valueByKeys(m map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func mapValueByKeys(m map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if mapped, ok := value.(map[string]any); ok {
				return mapped, true
			}
		}
	}
	return nil, false
}

func hasStringSliceContent(value any) bool {
	switch values := value.(type) {
	case []string:
		for _, item := range values {
			if strings.TrimSpace(item) != "" {
				return true
			}
		}
	case []any:
		for _, item := range values {
			if strings.TrimSpace(toString(item)) != "" {
				return true
			}
		}
	}
	return false
}

func toString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func detectConfigSource() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CONFIG_SOURCE"))) {
	case configSourceEtcd:
		return configSourceEtcd
	case configSourceFile:
		return configSourceFile
	}

	// 兼容旧逻辑：如果只设置了 MODE=prod，则默认走 etcd。
	switch runtimeAppEnv() {
	case "prod":
		return configSourceEtcd
	default:
		return configSourceFile
	}
}

func runtimeAppEnv() string {
	value := strings.ToLower(strings.TrimSpace(firstNonEmptyEnv("APP_ENV")))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(firstNonEmptyEnv("MODE")))
	}
	switch value {
	case "prod", "production":
		return "prod"
	case "test", "testing":
		return "test"
	case "debug", "dev", "development", "local":
		return "local"
	default:
		return "local"
	}
}

func runtimeMode(mode string) string {
	switch normalizeMode(mode) {
	case modeProd:
		return "pro"
	case modeTest:
		return "test"
	default:
		return "pro"
	}
}

func runtimeServiceName(service string) string {
	if name := strings.TrimSpace(os.Getenv("SERVICE_NAME")); name != "" {
		return name
	}
	if service == "gateway" {
		return "gateway"
	}
	if service == "update" {
		return "update-service"
	}
	return service + "-service"
}

func resolveServiceEtcdKey(service string, namespace string) string {
	return strings.TrimRight(namespace, "/") + "/services/" + service
}

func resolveSharedEtcdKey(namespace string) string {
	return strings.TrimRight(namespace, "/") + "/shared"
}

// resolveConfigRootNamespace 用于 etcd 读取配置。
// 新规范优先使用 CONFIG_NAMESPACE + APP_ENV，例如 /hfjj + prod => /hfjj/prod。
// 为兼容旧部署，也支持直接传 ETCD_NAMESPACE=/hfjj/prod。
func resolveConfigRootNamespace(options LoadOptions) string {
	if namespace := explicitEtcdNamespace(options); namespace != "" {
		return namespace
	}
	return "/hfjj/" + normalizeMode(options.Mode)
}

// resolveDiscoveryNamespace 用于服务注册发现。
// 正常情况下建议和配置根前缀保持同一环境层级。
func resolveDiscoveryNamespace(options LoadOptions) string {
	return resolveConfigRootNamespace(options)
}

func rpcKey(service string) string {
	if service == "captcha-solver" {
		return "captcha"
	}
	return service
}

func portEnvKey(service string) string {
	switch service {
	case "gateway":
		return "GATEWAY_PORT"
	case "passport":
		return "PASSPORT_PORT"
	case "school":
		return "SCHOOL_PORT"
	case "relation":
		return "RELATION_PORT"
	case "content":
		return "CONTENT_PORT"
	case "im":
		return "IM_PORT"
	case "captcha-solver":
		return "CAPTCHA_SOLVER_PORT"
	case "update":
		return "UPDATE_SERVICE_PORT"
	default:
		return "SERVICE_PORT"
	}
}

func defaultPort(service string) int {
	switch service {
	case "gateway":
		return 10000
	case "passport":
		return 20000
	case "school":
		return 30000
	case "relation":
		return 40000
	case "content":
		return 50000
	case "im":
		return 60000
	case "captcha-solver":
		return 61000
	case "update":
		return 62000
	default:
		return 0
	}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func envOrDefault(fallback string, names ...string) string {
	if value := firstNonEmptyEnv(names...); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func intFromEnv(name string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func normalizeHostsFields(v any) error {
	val := reflect.ValueOf(v)
	if !val.IsValid() || val.Kind() != reflect.Ptr || val.IsNil() {
		return nil
	}
	normalizeHostsValue(val.Elem())
	return nil
}

func normalizeHostsValue(val reflect.Value) {
	if !val.IsValid() {
		return
	}
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		normalizeHostsValue(val.Elem())
		return
	}
	switch val.Kind() {
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			if val.Type().Field(i).Name == "Hosts" && field.CanSet() && field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
				field.Set(reflect.ValueOf(splitCSV(strings.Join(stringSlice(field.Interface()), ","))))
				continue
			}
			normalizeHostsValue(field)
		}
	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			normalizeHostsValue(val.Index(i))
		}
	}
}

func stringSlice(value any) []string {
	if values, ok := value.([]string); ok {
		return values
	}
	return nil
}
