package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"hfjj/backend/pkg/constants"
)

// UserCache 用户信息缓存
type UserCache struct {
	cache *Cache
}

type CachedUser struct {
	ID            int64     `json:"id"`
	Account       string    `json:"account"`
	Nickname      string    `json:"nickname"`
	AvatarURL     string    `json:"avatar_url"`
	CoverImageURL string    `json:"cover_image_url"`
	Bio           string    `json:"bio"`
	AccountStatus int32     `json:"account_status"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewUserCache(cache *Cache) *UserCache {
	return &UserCache{cache: cache}
}

// Get 获取单个用户缓存
func (uc *UserCache) Get(ctx context.Context, userID int64) (*CachedUser, bool, error) {
	key := fmt.Sprintf("user:%d", userID)
	var user CachedUser
	found, err := uc.cache.Get(ctx, key, &user)
	if err != nil || !found {
		return nil, false, err
	}
	return &user, true, nil
}

// Set 设置用户缓存
func (uc *UserCache) Set(ctx context.Context, user *CachedUser) error {
	key := fmt.Sprintf("user:%d", user.ID)
	return uc.cache.Set(ctx, key, user, time.Duration(constants.CacheTTLUserInfo)*time.Second)
}

// MGet 批量获取用户缓存
func (uc *UserCache) MGet(ctx context.Context, userIDs []int64) (map[int64]*CachedUser, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*CachedUser), nil
	}

	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = fmt.Sprintf("user:%d", id)
	}

	data, err := uc.cache.MGet(ctx, keys)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*CachedUser, len(userIDs))
	for i, userID := range userIDs {
		if bytes, ok := data[keys[i]]; ok {
			var user CachedUser
			if err := json.Unmarshal(bytes, &user); err == nil {
				result[userID] = &user
			}
		}
	}

	return result, nil
}

// MSet 批量设置用户缓存
func (uc *UserCache) MSet(ctx context.Context, users []*CachedUser) error {
	if len(users) == 0 {
		return nil
	}

	items := make(map[string]interface{}, len(users))
	for _, user := range users {
		key := fmt.Sprintf("user:%d", user.ID)
		items[key] = user
	}

	return uc.cache.MSet(ctx, items, time.Duration(constants.CacheTTLUserInfo)*time.Second)
}

// Delete 删除用户缓存
func (uc *UserCache) Delete(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("user:%d", userID)
	return uc.cache.Delete(ctx, key)
}

// PostCache 帖子缓存
type PostCache struct {
	cache *Cache
}

func NewPostCache(cache *Cache) *PostCache {
	return &PostCache{cache: cache}
}

// Get 获取帖子缓存
func (pc *PostCache) Get(ctx context.Context, postID int64) (interface{}, bool, error) {
	key := fmt.Sprintf("post:%d", postID)
	var post interface{}
	found, err := pc.cache.Get(ctx, key, &post)
	return post, found, err
}

// Set 设置帖子缓存
func (pc *PostCache) Set(ctx context.Context, postID int64, post interface{}) error {
	key := fmt.Sprintf("post:%d", postID)
	return pc.cache.Set(ctx, key, post, time.Duration(constants.CacheTTLPostInfo)*time.Second)
}

// Delete 删除帖子缓存
func (pc *PostCache) Delete(ctx context.Context, postID int64) error {
	key := fmt.Sprintf("post:%d", postID)
	return pc.cache.Delete(ctx, key)
}

// HotTopicCache 热门话题缓存
type HotTopicCache struct {
	cache *Cache
}

func NewHotTopicCache(cache *Cache) *HotTopicCache {
	return &HotTopicCache{cache: cache}
}

// Get 获取热门话题列表
func (htc *HotTopicCache) Get(ctx context.Context) (interface{}, bool, error) {
	key := "hot_topics"
	var topics interface{}
	found, err := htc.cache.Get(ctx, key, &topics)
	return topics, found, err
}

// Set 设置热门话题列表
func (htc *HotTopicCache) Set(ctx context.Context, topics interface{}) error {
	key := "hot_topics"
	return htc.cache.Set(ctx, key, topics, time.Duration(constants.CacheTTLHotTopics)*time.Second)
}

// Delete 删除热门话题缓存
func (htc *HotTopicCache) Delete(ctx context.Context) error {
	return htc.cache.Delete(ctx, "hot_topics")
}
