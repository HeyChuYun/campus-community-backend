package backoffice

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/proto/passportpb"

	"github.com/redis/go-redis/v9"
)

var ErrInvalidInput = errors.New("invalid backoffice input")

type Service struct {
	repo        Repository
	redis       *redis.Client
	redisPrefix string
	statuses    map[int]struct{}
	rbac        *RBACService
	access      AccessPolicy
}

func NewService(repo Repository, redisClient *redis.Client, redisPrefix string, rbac *RBACService) *Service {
	return &Service{
		repo:        repo,
		redis:       requireRedisClient(redisClient),
		redisPrefix: redisPrefix,
		rbac:        rbac,
		statuses: map[int]struct{}{
			1: {},
			2: {},
			3: {},
			4: {},
		},
		access: DefaultAccessPolicy(),
	}
}

func (s *Service) RBAC() *RBACService {
	return s.rbac
}

func (s *Service) AllowsAccess(snapshot *passportpb.AuthorizationSnapshot) bool {
	if snapshot == nil {
		return false
	}
	return s.access.Allows(snapshot)
}

func (s *Service) RecordHeartbeat(ctx context.Context, input HeartbeatInput) error {
	if input.UserID <= 0 {
		return nil
	}
	now := time.Now().Unix()
	userID := strconv.FormatInt(input.UserID, 10)
	pipe := s.redis.Pipeline()
	pipe.ZAdd(ctx, s.redisPrefix+"online", redis.Z{Score: float64(now), Member: userID})
	pipe.ZAdd(ctx, s.redisPrefix+"online:recent", redis.Z{Score: float64(now), Member: userID})
	pipe.HSet(ctx, s.redisPrefix+"hb:"+userID,
		"version", input.Version,
		"platform", input.Platform,
		"device", input.Device,
		"ts", strconv.FormatInt(now, 10),
	)
	pipe.HSet(ctx, s.redisPrefix+"hb:recent:"+userID,
		"version", input.Version,
		"platform", input.Platform,
		"device", input.Device,
		"ts", strconv.FormatInt(now, 10),
	)
	pipe.Expire(ctx, s.redisPrefix+"hb:"+userID, heartbeatTTL)
	pipe.Expire(ctx, s.redisPrefix+"hb:recent:"+userID, recentHeartbeatTTL)
	pipe.ZRemRangeByScore(ctx, s.redisPrefix+"online", "0", strconv.FormatInt(now-onlineThreshold*3, 10))
	pipe.ZRemRangeByScore(ctx, s.redisPrefix+"online:recent", "0", strconv.FormatInt(now-int64(recentHeartbeatTTL.Seconds()), 10))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Service) DashboardStats(ctx context.Context, now time.Time) (DashboardStats, error) {
	stats := DashboardStats{}
	online, versionDist, platformDist, deviceDist := s.onlineStats(ctx, now)
	stats.OnlineCount = online
	if len(versionDist) == 0 || len(platformDist) == 0 || len(deviceDist) == 0 {
		recentVersion, recentPlatform, recentDevice := s.recentStats(ctx, now)
		if len(versionDist) == 0 {
			versionDist = recentVersion
		}
		if len(platformDist) == 0 {
			platformDist = recentPlatform
		}
		if len(deviceDist) == 0 {
			deviceDist = recentDevice
		}
	}
	stats.VersionDistribution = distributionToSlice(versionDist)
	stats.PlatformDistribution = distributionToSlice(platformDist)
	stats.DeviceDistribution = distributionToSlice(deviceDist)

	totalUsers, err := s.repo.CountUsers(ctx)
	if err != nil {
		return DashboardStats{}, err
	}
	stats.TotalUsers = totalUsers

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayNewUsers, err := s.repo.CountNewUsersSince(ctx, todayStart)
	if err != nil {
		return DashboardStats{}, err
	}
	stats.TodayNewUsers = todayNewUsers
	return stats, nil
}

func (s *Service) ListUsers(ctx context.Context, filter UserListFilter) (UserListResult, error) {
	return s.repo.ListUsers(ctx, filter)
}

func (s *Service) UpdateUserStatus(ctx context.Context, userID int64, status int) error {
	if userID <= 0 {
		return fmt.Errorf("%w: user id", ErrInvalidInput)
	}
	if _, ok := s.statuses[status]; !ok {
		return fmt.Errorf("%w: account status", ErrInvalidInput)
	}
	return s.repo.UpdateUserStatus(ctx, userID, status)
}

func (s *Service) UpsertUserRestriction(ctx context.Context, input UserRestrictionInput) error {
	if input.UserID <= 0 || input.OperatorUserID <= 0 {
		return fmt.Errorf("%w: user id", ErrInvalidInput)
	}
	if input.Duration <= 0 {
		return fmt.Errorf("%w: duration", ErrInvalidInput)
	}
	switch strings.TrimSpace(input.Scope) {
	case UserRestrictionScopePostPublish, UserRestrictionScopeDirectMessage, UserRestrictionScopeAccountLogin:
	default:
		return fmt.Errorf("%w: scope", ErrInvalidInput)
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		return fmt.Errorf("%w: reason", ErrInvalidInput)
	}
	return s.repo.UpsertUserRestriction(ctx, input)
}

func (s *Service) onlineStats(ctx context.Context, now time.Time) (int64, map[string]int, map[string]int, map[string]int) {
	versionDist := map[string]int{}
	platformDist := map[string]int{}
	deviceDist := map[string]int{}
	onlineIDs, err := s.redis.ZRangeByScore(ctx, s.redisPrefix+"online", &redis.ZRangeBy{
		Min: strconv.FormatInt(now.Unix()-onlineThreshold, 10),
		Max: "+inf",
	}).Result()
	if err != nil || len(onlineIDs) == 0 {
		return 0, versionDist, platformDist, deviceDist
	}

	pipe := s.redis.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(onlineIDs))
	for i, uid := range onlineIDs {
		cmds[i] = pipe.HGetAll(ctx, s.redisPrefix+"hb:"+uid)
	}
	_, _ = pipe.Exec(ctx)
	for _, cmd := range cmds {
		data := cmd.Val()
		incrementDistribution(versionDist, data["version"])
		incrementDistribution(platformDist, data["platform"])
		incrementDistribution(deviceDist, data["device"])
	}
	return int64(len(onlineIDs)), versionDist, platformDist, deviceDist
}

func (s *Service) recentStats(ctx context.Context, now time.Time) (map[string]int, map[string]int, map[string]int) {
	versionDist := map[string]int{}
	platformDist := map[string]int{}
	deviceDist := map[string]int{}
	userIDs, err := s.redis.ZRangeByScore(ctx, s.redisPrefix+"online:recent", &redis.ZRangeBy{
		Min: strconv.FormatInt(now.Add(-recentActiveWindow).Unix(), 10),
		Max: "+inf",
	}).Result()
	if err != nil || len(userIDs) == 0 {
		return versionDist, platformDist, deviceDist
	}

	pipe := s.redis.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(userIDs))
	for i, uid := range userIDs {
		cmds[i] = pipe.HGetAll(ctx, s.redisPrefix+"hb:recent:"+uid)
	}
	_, _ = pipe.Exec(ctx)
	for _, cmd := range cmds {
		data := cmd.Val()
		incrementDistribution(versionDist, data["version"])
		incrementDistribution(platformDist, data["platform"])
		incrementDistribution(deviceDist, data["device"])
	}
	return versionDist, platformDist, deviceDist
}

func incrementDistribution(dist map[string]int, key string) {
	key = strings.TrimSpace(key)
	if key != "" {
		dist[key]++
	}
}

func distributionToSlice(dist map[string]int) []DistributionItem {
	items := make([]DistributionItem, 0, len(dist))
	for name, count := range dist {
		items = append(items, DistributionItem{Name: name, Count: count})
	}
	return items
}

func requireRedisClient(client *redis.Client) *redis.Client {
	if client == nil {
		panic("backoffice: redis client is required")
	}
	return client
}
