package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type FavoritePostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFavoritePostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FavoritePostLogic {
	return &FavoritePostLogic{ctx: ctx, svcCtx: svcCtx}
}

// FavoritePost 收藏帖子，并在成功后补写一条 outbox 事件。
func (l *FavoritePostLogic) FavoritePost(in *contentpb.FavoritePostRequest) (*contentpb.FavoritePostResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.FavoritePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid favorite request")}, nil
	}

	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.FavoritePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found")}, nil
	}
	if status := checkPostVisibility(l.ctx, l.svcCtx, in.GetOperatorUserId(), post); status != nil {
		return &contentpb.FavoritePostResponse{Base: status}, nil
	}

	changed, err := l.svcCtx.FavStore.Favorite(l.ctx, in.GetOperatorUserId(), in.GetPostId())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := l.svcCtx.PostStore.IncrementFavoriteCount(l.ctx, in.GetPostId(), 1); err != nil {
			return nil, err
		}

		// 收藏成功后写入 outbox，供互动消息和兴趣画像异步消费。
		emitContentEvent(l.ctx, l.svcCtx, "content.post.favorited", in.GetOperatorUserId(), post.AuthorUserID, "post", post.ID, map[string]any{
			"post_id":             post.ID,
			"post_author_user_id": post.AuthorUserID,
			"favorited_user_id":   in.GetOperatorUserId(),
		})
	}

	return &contentpb.FavoritePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok")}, nil
}
