package config

import (
	"errors"
	"strings"

	"hfjj/backend/pkg/configx"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres  PostgresConfig
	AppRedis  gozeroredis.RedisKeyConf
	GeeTest   GeeTestConfig
	Token     TokenConfig
	SchoolRpc zrpc.RpcClientConf
}

type PostgresConfig struct {
	DSN string
}

type GeeTestConfig struct {
	Enabled       bool
	CaptchaID     string
	CaptchaKey    string
	ValidateURL   string
	TimeoutMillis int64
}

type TokenConfig struct {
	Issuer               string
	AccessSecret         string
	AccessExpireSeconds  int64
	RefreshExpireSeconds int64
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return errors.New("postgres dsn is empty")
	}
	if err := configx.ValidateRedis("passport redis", c.AppRedis); err != nil {
		return err
	}
	if c.Token.AccessExpireSeconds <= 0 || c.Token.RefreshExpireSeconds <= 0 {
		return errors.New("token expiry seconds must be positive")
	}
	if c.GeeTest.Enabled {
		if strings.TrimSpace(c.GeeTest.CaptchaID) == "" {
			return errors.New("geetest captcha id is empty")
		}
		if strings.TrimSpace(c.GeeTest.ValidateURL) == "" {
			return errors.New("geetest validate url is empty")
		}
		if c.GeeTest.TimeoutMillis <= 0 {
			return errors.New("geetest timeout millis must be positive")
		}
	}
	return configx.ValidateRPCClient("school rpc", c.SchoolRpc)
}
