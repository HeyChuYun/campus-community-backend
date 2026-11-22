package config

import (
	"errors"
	"strings"

	"hfjj/backend/pkg/configx"
	"hfjj/backend/pkg/outboxmq"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres     PostgresConfig
	AppRedis     gozeroredis.RedisKeyConf
	RabbitMQ     outboxmq.RabbitMQConfig
	Outbox       outboxmq.DispatcherConfig
}

type PostgresConfig struct {
	DSN string
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return errors.New("postgres dsn is empty")
	}
	if err := configx.ValidateRedis("relation redis", c.AppRedis); err != nil {
		return err
	}
	if c.RabbitMQ.Enabled && strings.TrimSpace(c.RabbitMQ.URL) == "" {
		return errors.New("rabbitmq url is empty")
	}
	if c.Outbox.Enabled {
		if c.Outbox.PollInterval <= 0 || c.Outbox.BatchSize <= 0 || c.Outbox.MaxRetry < 0 || c.Outbox.RetryBackoff <= 0 {
			return errors.New("outbox config is invalid")
		}
	}
	return nil
}
