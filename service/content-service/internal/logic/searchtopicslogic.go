package logic

import (
	"context"
	"strings"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type SearchTopicsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchTopicsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchTopicsLogic {
	return &SearchTopicsLogic{ctx: ctx, svcCtx: svcCtx}
}

// SearchTopics 按话题名搜索，分页大小固定在服务端。
func (l *SearchTopicsLogic) SearchTopics(in *contentpb.SearchTopicsRequest) (*contentpb.SearchTopicsResponse, error) {
	query := strings.TrimSpace(in.GetQuery())
	if query == "" {
		return &contentpb.SearchTopicsResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "empty query")}, nil
	}

	items, hasMore, err := l.svcCtx.TopicStore.SearchTopics(l.ctx, query, in.GetPage(), defaultTopicSearchPageSize)
	if err != nil {
		return nil, err
	}

	result := make([]*contentpb.TopicItem, 0, len(items))
	for _, item := range items {
		result = append(result, toTopicPB(item))
	}

	return &contentpb.SearchTopicsResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items:   result,
		HasMore: hasMore,
	}, nil
}
