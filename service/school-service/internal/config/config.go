package config

import (
	"errors"
	"strings"
	"time"

	"hfjj/backend/pkg/configx"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres     PostgresConfig
	AppRedis     gozeroredis.RedisKeyConf
	Security     SecurityConfig
	Campus       CampusConfig
	CaptchaRpc   zrpc.RpcClientConf
	LifecycleJob LifecycleJobConfig
}

type PostgresConfig struct {
	DSN string
}

type SecurityConfig struct {
	// SchoolPasswordKey 兼容保留字段，当前后端不再保存校园密码。
	SchoolPasswordKey string
}

type LifecycleJobConfig struct {
	Enabled    bool
	Interval   time.Duration
	LockTTL    time.Duration
	StartDelay time.Duration
	LockKey    string
}

type CampusConfig struct {
	VPNBaseURL         string
	EnlinkSSOLoginURL  string
	TargetServiceURL   string
	CASRoot            string
	JWBaseURL          string
	PersonalInfoURL    string
	ScheduleIndexURL   string
	ScheduleDataURL    string
	CaptchaFixedCode   string
	LoginToken         string
	LoginUserToken     string
	RSAExponentHex     string
	RSAModulusHex      string
	UserAgent          string
	InsecureSkipVerify bool
	Timeout            time.Duration
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return errors.New("postgres dsn is empty")
	}
	if err := configx.ValidateRedis("school redis", c.AppRedis); err != nil {
		return err
	}
	if c.LifecycleJob.Enabled {
		if c.LifecycleJob.Interval <= 0 || c.LifecycleJob.LockTTL <= 0 || c.LifecycleJob.StartDelay < 0 || strings.TrimSpace(c.LifecycleJob.LockKey) == "" {
			return errors.New("lifecycle job config is invalid")
		}
	}
	if strings.TrimSpace(c.Campus.VPNBaseURL) == "" ||
		strings.TrimSpace(c.Campus.RSAModulusHex) == "" ||
		strings.TrimSpace(c.Campus.TargetServiceURL) == "" {
		return errors.New("campus integration config is incomplete")
	}
	if c.Campus.Timeout <= 0 {
		return errors.New("campus timeout must be positive")
	}
	return configx.ValidateRPCClient("captcha rpc", c.CaptchaRpc)
}
