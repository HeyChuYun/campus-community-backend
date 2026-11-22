package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
	"hfjj/backend/proto/relationpb"
)

type GetFollowingFeedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFollowingFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFollowingFeedLogic {
	return &GetFollowingFeedLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetFollowingFeed 查询关注流，并允许看到自己设置为 followers only 的帖子。
func (l *GetFollowingFeedLogic) GetFollowingFeed(in *contentpb.GetFollowingFeedRequest) (*contentpb.GetFollowingFeedResponse, error) {
	if in.GetOperatorUserId() <= 0 {
		return &contentpb.GetFollowingFeedResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid operator_user_id")}, nil
	}
	if l.svcCtx.RelationRPC == nil {
		return &contentpb.GetFollowingFeedResponse{
			Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "relation rpc unavailable, return empty following feed"),
			Items:   []*contentpb.PostItem{},
			HasMore: false,
		}, nil
	}

	resp, err := l.svcCtx.RelationRPC.ListFollowing(l.ctx, &relationpb.ListFollowingRequest{
		UserId:   in.GetOperatorUserId(),
		Page:     1,
		PageSize: 500,
	})
	if err != nil {
		return nil, err
	}
	authorIDs := buildFollowingAuthorIDs(in.GetOperatorUserId(), resp)
	posts, hasMore, err := l.svcCtx.PostStore.ListPostsByAuthorIDs(l.ctx, in.GetOperatorUserId(), authorIDs, in.GetPage(), in.GetPageSize())
	if err != nil {
		return nil, err
	}
	return &contentpb.GetFollowingFeedResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items:   filterVisiblePosts(l.ctx, l.svcCtx, in.GetOperatorUserId(), in.GetCampusCode(), posts),
		HasMore: hasMore,
	}, nil
}
