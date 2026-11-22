package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type DeleteCommentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCommentLogic {
	return &DeleteCommentLogic{ctx: ctx, svcCtx: svcCtx}
}

// DeleteComment 软删除评论，并同步回写帖子评论计数。
func (l *DeleteCommentLogic) DeleteComment(in *contentpb.DeleteCommentRequest) (*contentpb.DeleteCommentResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetCommentId() <= 0 {
		return &contentpb.DeleteCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid delete comment request")}, nil
	}

	// 先读取评论，删除成功后需要知道它属于哪个帖子。
	comment, found, err := l.svcCtx.CommentStore.GetComment(l.ctx, in.GetCommentId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.DeleteCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "comment not found")}, nil
	}

	// 这里只允许评论作者删除自己的评论。
	deleted, err := l.svcCtx.CommentStore.DeleteComment(l.ctx, in.GetOperatorUserId(), in.GetCommentId())
	if err != nil {
		return nil, err
	}
	if !deleted {
		return &contentpb.DeleteCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "comment delete denied")}, nil
	}

	// 删除评论后同步扣减帖子评论数，避免评论计数漂移。
	if err := l.svcCtx.PostStore.IncrementCommentCount(l.ctx, comment.PostID, -1); err != nil {
		return nil, err
	}

	return &contentpb.DeleteCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok")}, nil
}
