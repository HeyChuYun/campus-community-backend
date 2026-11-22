package configx

import (
	"errors"
	"fmt"
	"strings"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type validator interface {
	Validate() error
}

func Validate(v any) error {
	if v == nil {
		return nil
	}
	if impl, ok := v.(validator); ok {
		return impl.Validate()
	}
	return nil
}

func ValidateRPCClient(name string, cfg zrpc.RpcClientConf) error {
	if strings.TrimSpace(cfg.Target) != "" {
		return nil
	}
	if len(cfg.Endpoints) > 0 {
		return nil
	}
	if hasNonEmpty(cfg.Etcd.Hosts) && strings.TrimSpace(cfg.Etcd.Key) != "" {
		return nil
	}
	return fmt.Errorf("%s rpc client must set target, endpoints, or etcd config", name)
}

func ValidateRedis(name string, cfg gozeroredis.RedisKeyConf) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%s redis host is empty", name)
	}
	return nil
}

func ValidateMinIO(endpoint, accessKeyID, secretAccessKey, bucket string) error {
	if strings.TrimSpace(endpoint) == "" {
		return errors.New("minio endpoint is empty")
	}
	if strings.TrimSpace(bucket) == "" {
		return errors.New("minio bucket is empty")
	}
	return nil
}

func hasNonEmpty(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
