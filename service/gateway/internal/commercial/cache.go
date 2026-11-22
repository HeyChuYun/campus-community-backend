package commercial

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	redis  *redis.Client
	prefix string
}

func NewCache(redisClient *redis.Client, prefix string) *Cache {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "gateway:"
	}
	return &Cache{redis: requireRedisClient(redisClient), prefix: prefix}
}

func (c *Cache) GetMembershipPolicy(ctx context.Context, userID int64) (MembershipPolicy, bool) {
	if userID <= 0 {
		return MembershipPolicy{}, false
	}
	value, err := c.redis.Get(ctx, c.membershipKey(userID)).Bytes()
	if err != nil {
		return MembershipPolicy{}, false
	}
	var policy MembershipPolicy
	if err := json.Unmarshal(value, &policy); err != nil {
		return MembershipPolicy{}, false
	}
	return policy, true
}

func (c *Cache) SetMembershipPolicy(ctx context.Context, userID int64, policy MembershipPolicy, ttl time.Duration) {
	if userID <= 0 || ttl <= 0 {
		return
	}
	value, err := json.Marshal(policy)
	if err != nil {
		return
	}
	_ = c.redis.Set(ctx, c.membershipKey(userID), value, ttl).Err()
}

func (c *Cache) InvalidateMembershipPolicy(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	_ = c.redis.Del(ctx, c.membershipKey(userID), "passport:authz:"+strconv.FormatInt(userID, 10)).Err()
}

func (c *Cache) PublishRBACReload(ctx context.Context) {
	channels := []string{"passport:casbin:reload", "casbin:reload", c.prefix + "casbin:reload"}
	seen := map[string]struct{}{}
	for _, channel := range channels {
		if channel == "" {
			continue
		}
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		_ = c.redis.Publish(ctx, channel, "reload").Err()
	}
}

func (c *Cache) membershipKey(userID int64) string {
	return c.prefix + "commercial:membership:policy:" + strconv.FormatInt(userID, 10)
}

func requireRedisClient(client *redis.Client) *redis.Client {
	if client == nil {
		panic("commercial: redis client is required")
	}
	return client
}
