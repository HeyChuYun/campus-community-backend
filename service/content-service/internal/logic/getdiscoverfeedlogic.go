package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type GetDiscoverFeedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDiscoverFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDiscoverFeedLogic {
	return &GetDiscoverFeedLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetDiscoverFeed 查询发现流，默认只返回公开帖子和自己的帖子。
func (l *GetDiscoverFeedLogic) GetDiscoverFeed(in *contentpb.GetDiscoverFeedRequest) (*contentpb.GetDiscoverFeedResponse, error) {
	posts, hasMore, err := l.svcCtx.PostStore.ListDiscoverPosts(l.ctx, in.GetOperatorUserId(), in.GetPage(), in.GetPageSize())
	if err != nil {
		return nil, err
	}
	return &contentpb.GetDiscoverFeedResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items:   filterVisiblePosts(l.ctx, l.svcCtx, in.GetOperatorUserId(), in.GetCampusCode(), posts),
		HasMore: hasMore,
	}, nil
}
