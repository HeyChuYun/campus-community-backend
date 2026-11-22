package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type ListDirectConversationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDirectConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDirectConversationsLogic {
	return &ListDirectConversationsLogic{ctx: ctx, svcCtx: svcCtx}
}

// ListDirectConversations returns the current user's direct-message
// conversations ordered by most recent activity.
func (l *ListDirectConversationsLogic) ListDirectConversations(in *impb.ListDirectConversationsRequest) (*impb.ListDirectConversationsResponse, error) {
	if in.GetUserId() <= 0 {
		return &impb.ListDirectConversationsResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "用户ID无效"),
		}, nil
	}

	items, hasMore, err := l.svcCtx.Repo.ListDirectConversations(l.ctx, in.GetUserId(), in.GetPage(), 20)
	if err != nil {
		return nil, err
	}

	result := make([]*impb.DirectConversationItem, 0, len(items))
	for _, item := range items {
		result = append(result, makeDirectConversationPB(item))
	}

	return &impb.ListDirectConversationsResponse{
		Base:    newStatus(impb.IMCode_IM_CODE_OK, "成功"),
		Items:   result,
		HasMore: hasMore,
	}, nil
}
