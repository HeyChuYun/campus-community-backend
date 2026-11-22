package logic

import (
	"context"

	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type GetPostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPostLogic {
	return &GetPostLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetPost 查询帖子详情，并处理 followers only 可见性。
func (l *GetPostLogic) GetPost(in *contentpb.GetPostRequest) (*contentpb.GetPostResponse, error) {
	if in.GetPostId() <= 0 {
		return &contentpb.GetPostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid post_id")}, nil
	}
	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.GetPostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found")}, nil
	}
	if status := checkPostVisibility(l.ctx, l.svcCtx, in.GetOperatorUserId(), post); status != nil {
		return &contentpb.GetPostResponse{Base: status}, nil
	}
	posts := []repository.Post{post}
	enrichPostsWithTopics(l.ctx, l.svcCtx, posts)
	post = posts[0]
	return &contentpb.GetPostResponse{
		Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Post: toPostPB(post),
	}, nil
}
