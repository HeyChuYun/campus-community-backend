package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type AuthorizationSnapshot struct {
	Roles           []string `json:"roles"`
	PermissionCodes []string `json:"permission_codes"`
}

type AuthorizationSnapshotStore interface {
	Get(userID int64) (AuthorizationSnapshot, bool, error)
	Put(userID int64, snapshot AuthorizationSnapshot, ttl time.Duration) error
	Delete(userID int64) error
}

type authorizationSnapshotStore struct {
	client  *redis.Client
	prefix  string
	mu      sync.RWMutex
	records map[string]AuthorizationSnapshot
}

// NewAuthorizationSnapshotStore 创建授权快照缓存。
// 这里只缓存“角色列表 + 隐式权限列表”，不缓存最终授权结果，避免资源级 own/any 判断被缓存误伤。
func NewAuthorizationSnapshotStore(client *redis.Client, prefix string) AuthorizationSnapshotStore {
	return &authorizationSnapshotStore{
		client:  client,
		prefix:  prefix + "authz:",
		records: make(map[string]AuthorizationSnapshot),
	}
}

func (s *authorizationSnapshotStore) Get(userID int64) (AuthorizationSnapshot, bool, error) {
	key := strconv.FormatInt(userID, 10)
	if s.client == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		record, ok := s.records[key]
		return record, ok, nil
	}

	raw, err := s.client.Get(context.Background(), s.prefix+key).Result()
	if err == redis.Nil {
		return AuthorizationSnapshot{}, false, nil
	}
	if err != nil {
		return AuthorizationSnapshot{}, false, err
	}

	var snapshot AuthorizationSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return AuthorizationSnapshot{}, false, err
	}
	return snapshot, true, nil
}

func (s *authorizationSnapshotStore) Put(userID int64, snapshot AuthorizationSnapshot, ttl time.Duration) error {
	key := strconv.FormatInt(userID, 10)
	if s.client == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.records[key] = snapshot
		return nil
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), s.prefix+key, payload, ttl).Err()
}

func (s *authorizationSnapshotStore) Delete(userID int64) error {
	key := strconv.FormatInt(userID, 10)
	if s.client == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.records, key)
		return nil
	}
	return s.client.Del(context.Background(), s.prefix+key).Err()
}
