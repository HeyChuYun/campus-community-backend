package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type UnfavoritePostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnfavoritePostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnfavoritePostLogic {
	return &UnfavoritePostLogic{ctx: ctx, svcCtx: svcCtx}
}

// UnfavoritePost 取消收藏帖子，并在成功后补写一条 outbox 事件。
func (l *UnfavoritePostLogic) UnfavoritePost(in *contentpb.UnfavoritePostRequest) (*contentpb.UnfavoritePostResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.UnfavoritePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid unfavorite request")}, nil
	}

	// 取消收藏前先读取帖子主体，后面写事件时需要带上帖子作者。
	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.UnfavoritePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found")}, nil
	}

	changed, err := l.svcCtx.FavStore.Unfavorite(l.ctx, in.GetOperatorUserId(), in.GetPostId())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := l.svcCtx.PostStore.IncrementFavoriteCount(l.ctx, in.GetPostId(), -1); err != nil {
			return nil, err
		}

		// 取消收藏成功后写入 outbox，供异步消费者撤销兴趣与互动状态。
		emitContentEvent(l.ctx, l.svcCtx, "content.post.unfavorited", in.GetOperatorUserId(), post.AuthorUserID, "post", post.ID, map[string]any{
			"post_id":             post.ID,
			"post_author_user_id": post.AuthorUserID,
			"unfavorited_user_id": in.GetOperatorUserId(),
		})
	}

	return &contentpb.UnfavoritePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok")}, nil
}
