package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type ListSystemNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSystemNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSystemNotificationsLogic {
	return &ListSystemNotificationsLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ListSystemNotificationsLogic) ListSystemNotifications(in *impb.ListSystemNotificationsRequest) (*impb.ListSystemNotificationsResponse, error) {
	if in.GetUserId() <= 0 {
		return &impb.ListSystemNotificationsResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid user id"),
		}, nil
	}

	items, hasMore, unreadCount, err := l.svcCtx.Repo.ListSystemNotifications(l.ctx, in.GetUserId(), in.GetPage(), 20)
	if err != nil {
		return nil, err
	}

	result := make([]*impb.SystemNotificationItem, 0, len(items))
	for _, item := range items {
		result = append(result, makeSystemNotificationPB(item))
	}

	return &impb.ListSystemNotificationsResponse{
		Base:        newStatus(impb.IMCode_IM_CODE_OK, "ok"),
		Items:       result,
		HasMore:     hasMore,
		UnreadCount: unreadCount,
	}, nil
}
