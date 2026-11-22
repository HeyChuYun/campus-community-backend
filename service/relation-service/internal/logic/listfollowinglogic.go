package logic

import (
	"context"

	"hfjj/backend/relation-service/internal/svc"
	"hfjj/backend/proto/relationpb"
)

type ListFollowingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFollowingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFollowingLogic {
	return &ListFollowingLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListFollowing 返回当前用户关注列表。
func (l *ListFollowingLogic) ListFollowing(in *relationpb.ListFollowingRequest) (*relationpb.ListFollowingResponse, error) {
	if in.GetUserId() <= 0 {
		return &relationpb.ListFollowingResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid user_id")}, nil
	}
	items, hasMore, err := l.svcCtx.RelationStore.ListFollowing(l.ctx, in.GetUserId(), in.GetPage(), in.GetPageSize())
	if err != nil {
		return nil, err
	}
	pbItems := make([]*relationpb.RelatedUserItem, 0, len(items))
	for _, item := range items {
		pbItems = append(pbItems, &relationpb.RelatedUserItem{
			UserId:        item.UserID,
			RelatedUserId: item.RelatedUserID,
			Cursor:        item.Cursor,
		})
	}
	return &relationpb.ListFollowingResponse{
		Base:    newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Items:   pbItems,
		HasMore: hasMore,
	}, nil
}
