package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	redis       *redis.Client
	globalLimit *rate.Limiter
	userLimits  sync.Map
	ipLimits    sync.Map
}

type userLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(redisClient *redis.Client, globalRPS, globalBurst int) *RateLimiter {
	rl := &RateLimiter{
		redis:       redisClient,
		globalLimit: rate.NewLimiter(rate.Limit(globalRPS), globalBurst),
	}

	// 定期清理过期的限流器
	go rl.cleanupExpiredLimiters()

	return rl
}

func (rl *RateLimiter) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 全局限流
			if !rl.globalLimit.Allow() {
				http.Error(w, `{"code":429,"message":"请求过于频繁，请稍后重试"}`, http.StatusTooManyRequests)
				return
			}

			// IP级限流
			clientIP := getClientIP(r)
			if !rl.allowIP(clientIP) {
				http.Error(w, `{"code":429,"message":"请求过于频繁，请稍后重试"}`, http.StatusTooManyRequests)
				return
			}

			// 用户级限流（如果已登录）
			userID := getUserIDFromRequest(r)
			if userID > 0 && !rl.allowUser(userID) {
				http.Error(w, `{"code":429,"message":"请求过于频繁，请稍后重试"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) allowIP(ip string) bool {
	limiter := rl.getIPLimiter(ip)
	return limiter.Allow()
}

func (rl *RateLimiter) allowUser(userID int64) bool {
	limiter := rl.getUserLimiter(userID)
	return limiter.Allow()
}

func (rl *RateLimiter) getIPLimiter(ip string) *rate.Limiter {
	if v, ok := rl.ipLimits.Load(ip); ok {
		ul := v.(*userLimiter)
		ul.lastSeen = time.Now()
		return ul.limiter
	}

	// IP限流：每秒20个请求，突发40
	limiter := rate.NewLimiter(20, 40)
	rl.ipLimits.Store(ip, &userLimiter{
		limiter:  limiter,
		lastSeen: time.Now(),
	})
	return limiter
}

func (rl *RateLimiter) getUserLimiter(userID int64) *rate.Limiter {
	key := fmt.Sprintf("user:%d", userID)
	if v, ok := rl.userLimits.Load(key); ok {
		ul := v.(*userLimiter)
		ul.lastSeen = time.Now()
		return ul.limiter
	}

	// 用户限流：每秒50个请求，突发100
	limiter := rate.NewLimiter(50, 100)
	rl.userLimits.Store(key, &userLimiter{
		limiter:  limiter,
		lastSeen: time.Now(),
	})
	return limiter
}

func (rl *RateLimiter) cleanupExpiredLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expiry := 10 * time.Minute

		rl.userLimits.Range(func(key, value interface{}) bool {
			ul := value.(*userLimiter)
			if now.Sub(ul.lastSeen) > expiry {
				rl.userLimits.Delete(key)
			}
			return true
		})

		rl.ipLimits.Range(func(key, value interface{}) bool {
			ul := value.(*userLimiter)
			if now.Sub(ul.lastSeen) > expiry {
				rl.ipLimits.Delete(key)
			}
			return true
		})
	}
}

// RedisRateLimiter 基于Redis的分布式限流器
type RedisRateLimiter struct {
	client *redis.Client
	prefix string
}

func NewRedisRateLimiter(client *redis.Client, prefix string) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		prefix: prefix,
	}
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if r.client == nil {
		return true, nil
	}

	fullKey := r.prefix + key
	now := time.Now().Unix()

	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, fullKey, "0", fmt.Sprintf("%d", now-int64(window.Seconds())))
	pipe.ZCard(ctx, fullKey)
	pipe.ZAdd(ctx, fullKey, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, fullKey, window)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := cmds[1].(*redis.IntCmd).Val()
	return count < limit, nil
}

func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func getUserIDFromRequest(r *http.Request) int64 {
	// 从context中获取用户ID（需要在auth中间件中设置）
	if userID, ok := r.Context().Value("user_id").(int64); ok {
		return userID
	}
	return 0
}
