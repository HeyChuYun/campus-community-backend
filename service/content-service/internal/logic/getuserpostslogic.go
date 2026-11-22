package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type GetUserPostsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserPostsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserPostsLogic {
	return &GetUserPostsLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetUserPosts 返回指定用户个人主页可见的帖子列表。
func (l *GetUserPostsLogic) GetUserPosts(in *contentpb.GetUserPostsRequest) (*contentpb.GetUserPostsResponse, error) {
	// 访问者和目标用户都必须明确，避免把权限判断留到后面才失败。
	if in.GetOperatorUserId() <= 0 || in.GetTargetUserId() <= 0 {
		return &contentpb.GetUserPostsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid user id"),
		}, nil
	}

	// 先按作者查询原始帖子，再根据访问者身份过滤 followers-only 帖子。
	posts, hasMore, err := l.svcCtx.PostStore.ListPostsByAuthorIDs(
		l.ctx,
		in.GetOperatorUserId(),
		[]int64{in.GetTargetUserId()},
		in.GetPage(),
		in.GetPageSize(),
	)
	if err != nil {
		return nil, err
	}

	// 可见性过滤统一复用现有逻辑，自己看自己时会保留全部正常帖子。
	items := filterVisiblePosts(l.ctx, l.svcCtx, in.GetOperatorUserId(), "all", posts)

	return &contentpb.GetUserPostsResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items:   items,
		HasMore: hasMore,
	}, nil
}
