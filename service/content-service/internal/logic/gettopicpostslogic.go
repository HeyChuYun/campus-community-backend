package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type GetTopicPostsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTopicPostsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTopicPostsLogic {
	return &GetTopicPostsLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetTopicPosts 按 topic_id 获取话题下的帖子列表。
// 这里直接走 post_topics 关联表，不再依赖正文关键字搜索，保证话题页结果准确。
func (l *GetTopicPostsLogic) GetTopicPosts(in *contentpb.GetTopicPostsRequest) (*contentpb.GetTopicPostsResponse, error) {
	if in.GetOperatorUserId() <= 0 {
		return &contentpb.GetTopicPostsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid operator_user_id"),
		}, nil
	}
	if in.GetTopicId() <= 0 {
		return &contentpb.GetTopicPostsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid topic_id"),
		}, nil
	}

	topic, found, err := l.svcCtx.TopicStore.GetTopic(l.ctx, in.GetTopicId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.GetTopicPostsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "topic not found"),
		}, nil
	}

	posts, hasMore, err := l.svcCtx.TopicStore.ListPostsByTopicID(
		l.ctx,
		in.GetTopicId(),
		in.GetPage(),
		defaultPostSearchPageSize,
	)
	if err != nil {
		return nil, err
	}

	return &contentpb.GetTopicPostsResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Topic:   toTopicPB(topic),
		Items:   filterVisiblePosts(l.ctx, l.svcCtx, in.GetOperatorUserId(), "all", posts),
		HasMore: hasMore,
	}, nil
}
