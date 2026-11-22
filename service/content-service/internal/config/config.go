package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"hfjj/backend/pkg/configx"
	"hfjj/backend/pkg/outboxmq"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres    PostgresConfig
	AppRedis    gozeroredis.RedisKeyConf
	RelationRpc zrpc.RpcClientConf
	Content     ContentConfig
	Moderation  ModerationConfig
	RabbitMQ    outboxmq.RabbitMQConfig
	Outbox      outboxmq.DispatcherConfig
}

type PostgresConfig struct {
	DSN string
}

type ContentConfig struct {
	Media MediaConfig
	Posts PostsConfig
	// Membership 可选；未配置时由业务代码内置默认会员策略。
	Membership MembershipConfig `json:",optional"`
}

type ModerationConfig struct {
	Enabled             bool
	Provider            string
	AccessKey           string
	SecretKey           string
	MediaBaseURL        string
	DefaultMuteDuration time.Duration
	RetryInterval       time.Duration
	MaxRetry            int
	PollBatchSize       int
	ReviewQueue         ReviewQueueConfig
	// CategoryRules 允许为空，空时走内置分类规则。
	CategoryRules []ModerationCategoryRule `json:",optional"`
}

type ReviewQueueConfig struct {
	Enabled       bool
	Queue         string
	ConsumerTag   string
	PrefetchCount int
	// Bindings 允许为空；仅在 ReviewQueue.Enabled=true 时由校验逻辑强约束。
	Bindings []string `json:",optional"`
}

type ModerationCategoryRule struct {
	Category string
	// Keywords 允许为空，空时该自定义分类规则不会生效。
	Keywords []string `json:",optional"`
}

type MediaConfig struct {
	MaxImageCount       int64
	MaxSingleImageBytes int64
	MaxVideoBytes       int64
	MaxGifBytes         int64
}

type PostsConfig struct {
	DailyPostLimit int64 `json:",default=3"`
}

type MembershipConfig struct {
	// Policies 允许为空，空时回退到系统内置会员策略。
	Policies []MembershipPolicy `json:",optional"`
}

type MembershipPolicy struct {
	PlanCode              string
	DailyPostBonus        int64
	ImageCountBonus       int64
	SingleImageBytesBonus int64
	VideoBytesBonus       int64
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return errors.New("postgres dsn is empty")
	}
	if err := configx.ValidateRedis("content redis", c.AppRedis); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("relation rpc", c.RelationRpc); err != nil {
		return err
	}
	if c.Content.Media.MaxImageCount <= 0 ||
		c.Content.Media.MaxSingleImageBytes <= 0 ||
		c.Content.Media.MaxVideoBytes <= 0 ||
		c.Content.Media.MaxGifBytes <= 0 {
		return fmt.Errorf("invalid content media limits")
	}
	if c.Content.Posts.DailyPostLimit <= 0 {
		return fmt.Errorf("invalid content daily post limit")
	}
	if c.Moderation.Enabled {
		if strings.TrimSpace(c.Moderation.Provider) == "" {
			return errors.New("moderation provider is empty")
		}
		if strings.TrimSpace(c.Moderation.AccessKey) == "" || strings.TrimSpace(c.Moderation.SecretKey) == "" {
			return errors.New("moderation credentials are incomplete")
		}
		if c.Moderation.DefaultMuteDuration <= 0 {
			return errors.New("moderation default mute duration must be positive")
		}
		if c.Moderation.RetryInterval <= 0 {
			return errors.New("moderation retry interval must be positive")
		}
		if c.Moderation.MaxRetry < 0 {
			return errors.New("moderation max retry must be non-negative")
		}
		if c.Moderation.PollBatchSize <= 0 {
			return errors.New("moderation poll batch size must be positive")
		}
		if c.Moderation.ReviewQueue.Enabled {
			if strings.TrimSpace(c.Moderation.ReviewQueue.Queue) == "" ||
				strings.TrimSpace(c.Moderation.ReviewQueue.ConsumerTag) == "" ||
				len(c.Moderation.ReviewQueue.Bindings) == 0 {
				return errors.New("moderation review queue config is incomplete")
			}
			if c.Moderation.ReviewQueue.PrefetchCount <= 0 {
				return errors.New("moderation queue prefetch count must be positive")
			}
		}
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
