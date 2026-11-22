package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type ListDirectMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDirectMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDirectMessagesLogic {
	return &ListDirectMessagesLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListDirectMessages returns the private message history between the current
// user and the specified peer.
func (l *ListDirectMessagesLogic) ListDirectMessages(in *impb.ListDirectMessagesRequest) (*impb.ListDirectMessagesResponse, error) {
	if in.GetUserId() <= 0 || in.GetPeerUserId() <= 0 {
		return &impb.ListDirectMessagesResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "用户ID无效"),
		}, nil
	}

	items, hasMore, err := l.svcCtx.Repo.ListDirectMessages(l.ctx, in.GetUserId(), in.GetPeerUserId(), in.GetPage(), 50)
	if err != nil {
		return nil, err
	}

	result := make([]*impb.DirectMessageItem, 0, len(items))
	for _, item := range items {
		result = append(result, makeDirectMessagePB(item))
	}

	return &impb.ListDirectMessagesResponse{
		Base:    newStatus(impb.IMCode_IM_CODE_OK, "成功"),
		Items:   result,
		HasMore: hasMore,
	}, nil
}
