package logic

import (
	"context"
	"strings"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

const (
	defaultPostSearchPageSize  int64 = 20
	defaultTopicSearchPageSize int64 = 20
	defaultHotTopicSize        int64 = 10
)

type SearchPostsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchPostsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchPostsLogic {
	return &SearchPostsLogic{ctx: ctx, svcCtx: svcCtx}
}

// SearchPosts 按正文内容搜索帖子，并沿用现有可见性过滤。
func (l *SearchPostsLogic) SearchPosts(in *contentpb.SearchPostsRequest) (*contentpb.SearchPostsResponse, error) {
	if in.GetOperatorUserId() <= 0 {
		return &contentpb.SearchPostsResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid operator_user_id")}, nil
	}
	query := strings.TrimSpace(in.GetQuery())
	if query == "" {
		return &contentpb.SearchPostsResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "empty query")}, nil
	}

	posts, hasMore, err := l.svcCtx.PostStore.SearchPosts(l.ctx, query, in.GetPage(), defaultPostSearchPageSize)
	if err != nil {
		return nil, err
	}

	return &contentpb.SearchPostsResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items:   filterVisiblePosts(l.ctx, l.svcCtx, in.GetOperatorUserId(), "all", posts),
		HasMore: hasMore,
	}, nil
}
