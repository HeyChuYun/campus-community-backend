package logic

import (
	"context"

	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type CreateCommentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCommentLogic {
	return &CreateCommentLogic{ctx: ctx, svcCtx: svcCtx}
}

// CreateComment 创建评论，并在成功后补写一条 outbox 事件。
func (l *CreateCommentLogic) CreateComment(in *contentpb.CreateCommentRequest) (*contentpb.CreateCommentResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 || cleanText(in.GetContent()) == "" {
		return &contentpb.CreateCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid comment request")}, nil
	}
	if containsUnsafeScriptPattern(in.GetContent()) {
		return &contentpb.CreateCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "comment content contains unsafe script pattern")}, nil
	}

	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.CreateCommentResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found")}, nil
	}
	if status := checkPostVisibility(l.ctx, l.svcCtx, in.GetOperatorUserId(), post); status != nil {
		return &contentpb.CreateCommentResponse{Base: status}, nil
	}

	comment, err := l.svcCtx.CommentStore.CreateComment(l.ctx, repository.Comment{
		PostID:           in.GetPostId(),
		UserID:           in.GetOperatorUserId(),
		ReplyToCommentID: in.GetReplyToCommentId(),
		ReplyToUserID:    in.GetReplyToUserId(),
		Content:          cleanText(in.GetContent()),
	})
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.PostStore.IncrementCommentCount(l.ctx, in.GetPostId(), 1); err != nil {
		return nil, err
	}

	// 评论成功后写入 outbox，后续可由 IM 服务消费成互动消息。
	emitContentEvent(l.ctx, l.svcCtx, "content.post.commented", in.GetOperatorUserId(), post.AuthorUserID, "comment", comment.ID, map[string]any{
		"post_id":             post.ID,
		"post_author_user_id": post.AuthorUserID,
		"comment":             toCommentPB(comment),
	})

	return &contentpb.CreateCommentResponse{
		Base:    newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Comment: toCommentPB(comment),
	}, nil
}
