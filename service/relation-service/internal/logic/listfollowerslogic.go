package logic

import (
	"context"

	"hfjj/backend/relation-service/internal/svc"
	"hfjj/backend/proto/relationpb"
)

type ListFollowersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFollowersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFollowersLogic {
	return &ListFollowersLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListFollowers 返回当前用户粉丝列表。
func (l *ListFollowersLogic) ListFollowers(in *relationpb.ListFollowersRequest) (*relationpb.ListFollowersResponse, error) {
	if in.GetUserId() <= 0 {
		return &relationpb.ListFollowersResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid user_id")}, nil
	}
	items, hasMore, err := l.svcCtx.RelationStore.ListFollowers(l.ctx, in.GetUserId(), in.GetPage(), in.GetPageSize())
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
	return &relationpb.ListFollowersResponse{
		Base:    newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Items:   pbItems,
		HasMore: hasMore,
	}, nil
}
