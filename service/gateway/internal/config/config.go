package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"hfjj/backend/pkg/configx"
	"hfjj/backend/pkg/outboxmq"

	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Postgres         PostgresConfig
	PassportJwt      PassportJwtConfig
	Redis            gozeroredis.RedisKeyConf
	Replay           ReplayConfig
	CampusCredential CampusCredentialConfig
	MediaStorage     MediaStorageConfig
	Payment          PaymentConfig
	Commercial       CommercialConfig
	RabbitMQ         outboxmq.RabbitMQConfig
	UpdateRpc        zrpc.RpcClientConf
	PassportRpc      zrpc.RpcClientConf
	SchoolRpc        zrpc.RpcClientConf
	CaptchaRpc       zrpc.RpcClientConf
	RelationRpc      zrpc.RpcClientConf
	ContentRpc       zrpc.RpcClientConf
	ImRpc            zrpc.RpcClientConf
}

// PostgresConfig PGsql数据库配置
type PostgresConfig struct {
	DSN string
}

type PassportJwtConfig struct {
	AccessSecret string
}

type ReplayConfig struct {
	SkewSeconds     int64 `json:",default=300"`
	NonceTTLSeconds int64 `json:",default=300"`
}

type CampusCredentialConfig struct {
	MasterKey string
	TokenTTL  time.Duration `json:",default=2160h"`
}

type MediaStorageConfig struct {
	Type      string `json:",default=minio,options=[minio]"`
	PublicURL string `json:",optional"`
	MinIO     MinIOStorageConfig
}

type MinIOStorageConfig struct {
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	Bucket           string
	UseSSL           bool
	Region           string `json:",optional"`
	ObjectPrefix     string `json:",default=media"`
	AutoCreateBucket bool   `json:",default=true"`
}

type PaymentConfig struct {
	Alipay PaymentAlipayConfig
	Wechat PaymentWechatConfig
}

type PaymentAlipayConfig struct {
	AppID           string
	PrivateKey      string
	AlipayPublicKey string
	NotifyURL       string
	ReturnURL       string
	Sandbox         bool `json:",default=true"`
}

type PaymentWechatConfig struct {
	AppID             string
	MchID             string
	APIv3Key          string
	SerialNo          string
	PrivateKey        string
	PlatformPublicKey string
	NotifyURL         string
	Sandbox           bool `json:",default=true"`
}

type CommercialConfig struct {
	Advertisement AdvertisementConfig
	// Membership 可选；未配置时由商业模块内部使用默认会员策略。
	Membership MembershipConfig `json:",optional"`
}

type AdvertisementConfig struct {
	AmountCents          int `json:",default=30000"`
	DurationDays         int `json:",default=30"`
	FirstAdAfterPosts    int `json:",default=7"`
	NextAdMinPosts       int `json:",default=10"`
	NextAdMaxPosts       int `json:",default=14"`
	MaxAdsPerPage        int `json:",default=1"`
	MaxAdsPer50Posts     int `json:",default=3"`
	SameAdCooldownHours  int `json:",default=6"`
	LightReductionAfter  int `json:",default=12"`
	CandidateCacheSecond int `json:",default=30"`
}

type MembershipConfig struct {
	// Policies 允许为空，空时回退到系统内置会员策略。
	Policies []MembershipPolicyConfig `json:",optional"`
}

type MembershipPolicyConfig struct {
	PlanCode              string
	DailyPostBonus        int64
	ImageCountBonus       int64
	SingleImageBytesBonus int64
	VideoBytesBonus       int64
	AdPolicy              string
	MediaQueuePriority    int
}

func (c Config) Validate() error {
	if err := configx.ValidateRedis("gateway redis", c.Redis); err != nil {
		return err
	}
	if strings.TrimSpace(c.MediaStorage.Type) == "" {
		return errors.New("media storage type is empty")
	}
	switch c.MediaStorage.Type {
	case "minio":
		if err := configx.ValidateMinIO(
			c.MediaStorage.MinIO.Endpoint,
			c.MediaStorage.MinIO.AccessKeyID,
			c.MediaStorage.MinIO.SecretAccessKey,
			c.MediaStorage.MinIO.Bucket,
		); err != nil {
			return fmt.Errorf("media storage minio config invalid: %w", err)
		}
	default:
		return fmt.Errorf("unsupported media storage type %q", c.MediaStorage.Type)
	}
	if err := configx.ValidateRPCClient("update rpc", c.UpdateRpc); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("passport rpc", c.PassportRpc); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("school rpc", c.SchoolRpc); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("captcha rpc", c.CaptchaRpc); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("relation rpc", c.RelationRpc); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("content rpc", c.ContentRpc); err != nil {
		return err
	}
	if err := configx.ValidateRPCClient("im rpc", c.ImRpc); err != nil {
		return err
	}
	if c.RabbitMQ.Enabled && strings.TrimSpace(c.RabbitMQ.URL) == "" {
		return errors.New("rabbitmq url is empty")
	}
	return nil
}
