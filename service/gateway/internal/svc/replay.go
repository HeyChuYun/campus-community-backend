package svc

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type ReplayProtector interface {
	CheckAndStore(scope string, ttl time.Duration) (bool, error)
}

type replayProtector struct {
	redis  *redis.Client
	prefix string
}

func NewReplayProtector(redisClient *redis.Client, prefix string) ReplayProtector {
	return &replayProtector{
		redis:  requireReplayRedis(redisClient),
		prefix: prefix + "replay:",
	}
}

func (r *replayProtector) CheckAndStore(scope string, ttl time.Duration) (bool, error) {
	result, err := r.redis.SetNX(context.Background(), r.prefix+scope, strconv.FormatInt(time.Now().UnixNano(), 10), ttl).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func requireReplayRedis(client *redis.Client) *redis.Client {
	if client == nil {
		panic("gateway replay protector requires redis client")
	}
	return client
}
