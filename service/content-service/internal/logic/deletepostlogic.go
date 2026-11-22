package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type DeletePostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeletePostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletePostLogic {
	return &DeletePostLogic{ctx: ctx, svcCtx: svcCtx}
}

// DeletePost 软删除帖子，并在删除成功后补写一条 outbox 事件。
func (l *DeletePostLogic) DeletePost(in *contentpb.DeletePostRequest) (*contentpb.DeletePostResponse, error) {
	// 先校验基础入参，避免无效请求进入存储层。
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.DeletePostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid delete post request"),
		}, nil
	}

	// 删除前先读取帖子主体，后面写事件时需要作者和帖子标识。
	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.DeletePostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found"),
		}, nil
	}

	// 非强制删除只允许作者本人操作，管理员强制删除由网关先做 delete.any 鉴权。
	if !in.GetForceDelete() && post.AuthorUserID != in.GetOperatorUserId() {
		return &contentpb.DeletePostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "cannot delete other user's post"),
		}, nil
	}

	// 执行数据库软删除；数据库模式和内存模式都会返回是否真的删除成功。
	deleted, err := l.svcCtx.PostStore.DeletePost(
		l.ctx,
		in.GetOperatorUserId(),
		in.GetPostId(),
		in.GetForceDelete(),
	)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return &contentpb.DeletePostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "delete post denied"),
		}, nil
	}

	// 删除成功后写入 outbox，供异步索引、通知或审计链路消费。
	emitContentEvent(l.ctx, l.svcCtx, "content.post.deleted", in.GetOperatorUserId(), post.AuthorUserID, "post", post.ID, map[string]any{
		"post_id":          post.ID,
		"author_user_id":   post.AuthorUserID,
		"operator_user_id": in.GetOperatorUserId(),
		"force_delete":     in.GetForceDelete(),
	})

	return &contentpb.DeletePostResponse{
		Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
	}, nil
}
