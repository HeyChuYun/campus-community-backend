package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type LikePostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLikePostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LikePostLogic {
	return &LikePostLogic{ctx: ctx, svcCtx: svcCtx}
}

// LikePost 点赞帖子，并在成功后补写一条 outbox 事件。
func (l *LikePostLogic) LikePost(in *contentpb.LikePostRequest) (*contentpb.LikePostResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.LikePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid like request")}, nil
	}

	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.LikePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found")}, nil
	}
	if status := checkPostVisibility(l.ctx, l.svcCtx, in.GetOperatorUserId(), post); status != nil {
		return &contentpb.LikePostResponse{Base: status}, nil
	}

	changed, err := l.svcCtx.LikeStore.Like(l.ctx, in.GetOperatorUserId(), in.GetPostId())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := l.svcCtx.PostStore.IncrementLikeCount(l.ctx, in.GetPostId(), 1); err != nil {
			return nil, err
		}

		// 点赞成功后写入 outbox，供互动消息和热度系统异步消费。
		emitContentEvent(l.ctx, l.svcCtx, "content.post.liked", in.GetOperatorUserId(), post.AuthorUserID, "post", post.ID, map[string]any{
			"post_id":             post.ID,
			"post_author_user_id": post.AuthorUserID,
			"liked_user_id":       in.GetOperatorUserId(),
		})
	}

	return &contentpb.LikePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok")}, nil
}
