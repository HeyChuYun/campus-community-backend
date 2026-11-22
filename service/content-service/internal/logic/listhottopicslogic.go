package logic

import (
	"context"
	"strconv"
	"time"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

const (
	hotTopicRefreshInterval = 24 * time.Hour
	hotTopicRefreshLockTTL  = 2 * time.Minute
	hotTopicLockKey         = "hot_topics:refresh_lock"
	hotTopicRefreshAtKey    = "hot_topics:last_refresh_at"
)

type ListHotTopicsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListHotTopicsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHotTopicsLogic {
	return &ListHotTopicsLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListHotTopics 返回热点话题。
// 调用前会先检查 Redis 中的刷新时间，过期时尝试抢锁并重算一次。
func (l *ListHotTopicsLogic) ListHotTopics(_ *contentpb.ListHotTopicsRequest) (*contentpb.ListHotTopicsResponse, error) {
	if err := l.tryRefreshHotTopics(); err != nil {
		return nil, err
	}

	items, err := l.svcCtx.TopicStore.ListHotTopics(l.ctx, defaultHotTopicSize)
	if err != nil {
		return nil, err
	}

	result := make([]*contentpb.TopicItem, 0, len(items))
	for _, item := range items {
		result = append(result, toHotTopicPB(item))
	}

	return &contentpb.ListHotTopicsResponse{
		Base:  newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items: result,
	}, nil
}

func (l *ListHotTopicsLogic) tryRefreshHotTopics() error {
	if l.svcCtx == nil || l.svcCtx.TopicStore == nil {
		return nil
	}

	// 没有 Redis 时退化为直接重算，保证本地环境也能看到热点。
	if l.svcCtx.Redis == nil {
		return l.refreshHotTopicsNow()
	}

	lastRefreshRaw, err := l.svcCtx.Redis.Get(l.ctx, l.svcCtx.Config.AppRedis.Key+hotTopicRefreshAtKey).Result()
	if err == nil {
		if unixSeconds, parseErr := strconv.ParseInt(lastRefreshRaw, 10, 64); parseErr == nil {
			lastRefresh := time.Unix(unixSeconds, 0)
			if time.Since(lastRefresh) < hotTopicRefreshInterval {
				return nil
			}
		}
	}

	locked, err := l.svcCtx.Redis.SetNX(l.ctx, l.svcCtx.Config.AppRedis.Key+hotTopicLockKey, strconv.FormatInt(time.Now().Unix(), 10), hotTopicRefreshLockTTL).Result()
	if err != nil {
		return nil
	}
	if !locked {
		return nil
	}
	defer l.svcCtx.Redis.Del(context.Background(), l.svcCtx.Config.AppRedis.Key+hotTopicLockKey)

	if err := l.refreshHotTopicsNow(); err != nil {
		return err
	}

	return l.svcCtx.Redis.Set(l.ctx, l.svcCtx.Config.AppRedis.Key+hotTopicRefreshAtKey, strconv.FormatInt(time.Now().Unix(), 10), 0).Err()
}

func (l *ListHotTopicsLogic) refreshHotTopicsNow() error {
	items, err := l.svcCtx.TopicStore.BuildHotTopics(l.ctx, defaultHotTopicSize)
	if err != nil {
		return err
	}
	return l.svcCtx.TopicStore.ReplaceHotTopics(l.ctx, items)
}
