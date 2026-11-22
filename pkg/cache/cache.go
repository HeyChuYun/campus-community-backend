package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	prefix string
}

func NewCache(client *redis.Client, prefix string) *Cache {
	return &Cache{
		client: requireRedisClient(client),
		prefix: prefix,
	}
}

// Get 获取缓存
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	fullKey := c.prefix + key
	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}

	return true, nil
}

// Set 设置缓存
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := c.prefix + key
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, fullKey, data, ttl).Err()
}

// Delete 删除缓存
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = c.prefix + key
	}

	return c.client.Del(ctx, fullKeys...).Err()
}

// MGet 批量获取缓存
func (c *Cache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = c.prefix + key
	}

	values, err := c.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte, len(keys))
	for i, val := range values {
		if val != nil {
			if str, ok := val.(string); ok {
				result[keys[i]] = []byte(str)
			}
		}
	}

	return result, nil
}

// MSet 批量设置缓存
func (c *Cache) MSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for key, value := range items {
		fullKey := c.prefix + key
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		pipe.Set(ctx, fullKey, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Exists 检查键是否存在
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := c.prefix + key
	count, err := c.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Expire 设置过期时间
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := c.prefix + key
	return c.client.Expire(ctx, fullKey, ttl).Err()
}

// Incr 自增
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	fullKey := c.prefix + key
	return c.client.Incr(ctx, fullKey).Result()
}

// Decr 自减
func (c *Cache) Decr(ctx context.Context, key string) (int64, error) {
	fullKey := c.prefix + key
	return c.client.Decr(ctx, fullKey).Result()
}

func requireRedisClient(client *redis.Client) *redis.Client {
	if client == nil {
		panic("pkg/cache: redis client is required")
	}
	return client
}
