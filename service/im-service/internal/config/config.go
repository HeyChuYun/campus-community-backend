package config

import (
	"errors"
	"strings"
	"time"

	"hfjj/backend/pkg/configx"
	"hfjj/backend/pkg/outboxmq"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres      PostgresConfig
	RelationRpc   zrpc.RpcClientConf
	RabbitMQ      outboxmq.RabbitMQConfig
	Consumer      ConsumerConfig
	DirectMessage DirectMessageConfig
	WuKong        WuKongConfig
}

type PostgresConfig struct {
	DSN string
}

type ConsumerConfig struct {
	Enabled       bool
	Queue         string
	ConsumerTag   string
	PrefetchCount int
	Bindings      []string
}

type DirectMessageConfig struct {
	NonMutualLimit int64
}

type WuKongConfig struct {
	BaseURL                     string
	AdminToken                  string
	TokenUpdatePath             string
	RoutePath                   string
	MessageSendPath             string
	ConversationClearUnreadPath string
	PublicTCPAddr               string
	PublicWSAddr                string
	PublicWSSAddr               string
	RouteIntranet               int
	SystemUID                   string
	TokenTTL                    time.Duration
	Timeout                     time.Duration
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return errors.New("postgres dsn is empty")
	}
	if err := configx.ValidateRPCClient("relation rpc", c.RelationRpc); err != nil {
		return err
	}
	if c.Consumer.Enabled {
		if strings.TrimSpace(c.Consumer.Queue) == "" || strings.TrimSpace(c.Consumer.ConsumerTag) == "" || len(c.Consumer.Bindings) == 0 {
			return errors.New("consumer config is incomplete")
		}
		if c.Consumer.PrefetchCount <= 0 {
			return errors.New("consumer prefetch count must be positive")
		}
	}
	if strings.TrimSpace(c.WuKong.BaseURL) == "" || c.WuKong.Timeout <= 0 {
		return errors.New("wukong config is incomplete")
	}
	if c.DirectMessage.NonMutualLimit < 0 {
		return errors.New("direct message limit cannot be negative")
	}
	if c.RabbitMQ.Enabled && strings.TrimSpace(c.RabbitMQ.URL) == "" {
		return errors.New("rabbitmq url is empty")
	}
	return nil
}
