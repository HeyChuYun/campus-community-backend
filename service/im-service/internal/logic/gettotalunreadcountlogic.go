package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type GetTotalUnreadCountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTotalUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTotalUnreadCountLogic {
	return &GetTotalUnreadCountLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *GetTotalUnreadCountLogic) GetTotalUnreadCount(in *impb.GetTotalUnreadCountRequest) (*impb.GetTotalUnreadCountResponse, error) {
	directUnread, err := l.svcCtx.Repo.GetTotalUnreadDirectMessageCount(l.ctx, in.UserId)
	if err != nil {
		return nil, err
	}

	interactionUnread, err := l.svcCtx.Repo.CountUnreadInteractions(l.ctx, in.UserId)
	if err != nil {
		return nil, err
	}

	systemUnread, err := l.svcCtx.Repo.CountUnreadSystemNotifications(l.ctx, in.UserId)
	if err != nil {
		return nil, err
	}

	return &impb.GetTotalUnreadCountResponse{
		Base: &impb.BaseResp{
			Code:    impb.IMCode_IM_CODE_OK,
			Message: "成功",
		},
		DirectMessageUnread:      directUnread,
		InteractionMessageUnread: interactionUnread,
		SystemNotificationUnread: systemUnread,
		TotalUnread:              directUnread + interactionUnread + systemUnread,
	}, nil
}
