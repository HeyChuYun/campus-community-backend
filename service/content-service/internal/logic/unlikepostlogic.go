package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type UnlikePostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnlikePostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnlikePostLogic {
	return &UnlikePostLogic{ctx: ctx, svcCtx: svcCtx}
}

// UnlikePost 取消点赞帖子，并在成功后补写一条 outbox 事件。
func (l *UnlikePostLogic) UnlikePost(in *contentpb.UnlikePostRequest) (*contentpb.UnlikePostResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.UnlikePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid unlike request")}, nil
	}

	// 取消点赞前先读取帖子主体，后面写事件时需要带上帖子作者。
	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.UnlikePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found")}, nil
	}

	changed, err := l.svcCtx.LikeStore.Unlike(l.ctx, in.GetOperatorUserId(), in.GetPostId())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := l.svcCtx.PostStore.IncrementLikeCount(l.ctx, in.GetPostId(), -1); err != nil {
			return nil, err
		}

		// 取消点赞成功后写入 outbox，供异步消费者撤销互动状态。
		emitContentEvent(l.ctx, l.svcCtx, "content.post.unliked", in.GetOperatorUserId(), post.AuthorUserID, "post", post.ID, map[string]any{
			"post_id":             post.ID,
			"post_author_user_id": post.AuthorUserID,
			"unliked_user_id":     in.GetOperatorUserId(),
		})
	}

	return &contentpb.UnlikePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok")}, nil
}
