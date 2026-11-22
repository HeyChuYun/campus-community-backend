package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type ListCommentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCommentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCommentsLogic {
	return &ListCommentsLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListComments 查询指定帖子下的评论列表。
// 这里先校验帖子是否存在，再复用帖子可见性校验，避免越权读取仅关注可见帖子的评论。
func (l *ListCommentsLogic) ListComments(in *contentpb.ListCommentsRequest) (*contentpb.ListCommentsResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.ListCommentsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid list comments request"),
		}, nil
	}

	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.ListCommentsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found"),
		}, nil
	}
	if status := checkPostVisibility(l.ctx, l.svcCtx, in.GetOperatorUserId(), post); status != nil {
		return &contentpb.ListCommentsResponse{Base: status}, nil
	}

	comments, hasMore, err := l.svcCtx.CommentStore.ListCommentsByPostID(
		l.ctx,
		in.GetPostId(),
		in.GetPage(),
		in.GetPageSize(),
	)
	if err != nil {
		return nil, err
	}

	items := make([]*contentpb.CommentItem, 0, len(comments))
	for _, comment := range comments {
		items = append(items, toCommentPB(comment))
	}

	return &contentpb.ListCommentsResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Items:   items,
		HasMore: hasMore,
	}, nil
}
