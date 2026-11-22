package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type ListInteractionMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListInteractionMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInteractionMessagesLogic {
	return &ListInteractionMessagesLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListInteractionMessages 负责查询互动消息列表和当前未读数。
func (l *ListInteractionMessagesLogic) ListInteractionMessages(in *impb.ListInteractionMessagesRequest) (*impb.ListInteractionMessagesResponse, error) {
	if in.GetUserId() <= 0 {
		return &impb.ListInteractionMessagesResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid user id"),
		}, nil
	}

	items, hasMore, unreadCount, err := l.svcCtx.Repo.ListInteractionMessages(l.ctx, in.GetUserId(), in.GetPage(), 20)
	if err != nil {
		return nil, err
	}

	result := make([]*impb.InteractionMessageItem, 0, len(items))
	for _, item := range items {
		result = append(result, makeInteractionPB(item))
	}

	return &impb.ListInteractionMessagesResponse{
		Base:       newStatus(impb.IMCode_IM_CODE_OK, "ok"),
		Items:      result,
		HasMore:    hasMore,
		UnreadCount: unreadCount,
	}, nil
}
