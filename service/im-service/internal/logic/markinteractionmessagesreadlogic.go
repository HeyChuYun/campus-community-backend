package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type MarkInteractionMessagesReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkInteractionMessagesReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkInteractionMessagesReadLogic {
	return &MarkInteractionMessagesReadLogic{ctx: ctx, svcCtx: svcCtx}
}

// MarkInteractionMessagesRead 负责批量把互动消息标记为已读。
func (l *MarkInteractionMessagesReadLogic) MarkInteractionMessagesRead(in *impb.MarkInteractionMessagesReadRequest) (*impb.MarkInteractionMessagesReadResponse, error) {
	if in.GetUserId() <= 0 {
		return &impb.MarkInteractionMessagesReadResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid user id"),
		}, nil
	}

	unreadCount, err := l.svcCtx.Repo.MarkInteractionMessagesRead(l.ctx, in.GetUserId(), in.GetInteractionIds())
	if err != nil {
		return nil, err
	}

	return &impb.MarkInteractionMessagesReadResponse{
		Base:       newStatus(impb.IMCode_IM_CODE_OK, "ok"),
		UnreadCount: unreadCount,
	}, nil
}
