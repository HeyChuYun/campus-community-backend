package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type MarkSystemNotificationsReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkSystemNotificationsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkSystemNotificationsReadLogic {
	return &MarkSystemNotificationsReadLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *MarkSystemNotificationsReadLogic) MarkSystemNotificationsRead(in *impb.MarkSystemNotificationsReadRequest) (*impb.MarkSystemNotificationsReadResponse, error) {
	if in.GetUserId() <= 0 {
		return &impb.MarkSystemNotificationsReadResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid user id"),
		}, nil
	}

	unreadCount, err := l.svcCtx.Repo.MarkSystemNotificationsRead(l.ctx, in.GetUserId(), in.GetNotificationIds())
	if err != nil {
		return nil, err
	}

	return &impb.MarkSystemNotificationsReadResponse{
		Base:        newStatus(impb.IMCode_IM_CODE_OK, "ok"),
		UnreadCount: unreadCount,
	}, nil
}
