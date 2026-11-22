package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/im-service/internal/wukong"
	"hfjj/backend/proto/impb"
)

type MarkConversationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkConversationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkConversationReadLogic {
	return &MarkConversationReadLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *MarkConversationReadLogic) MarkConversationRead(in *impb.MarkConversationReadRequest) (*impb.MarkConversationReadResponse, error) {
	if in.GetUserId() <= 0 || in.GetChannelId() == "" {
		return &impb.MarkConversationReadResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid read request"),
		}, nil
	}

	if in.GetConversationType() == impb.ConversationType_CONVERSATION_TYPE_PERSONAL {
		peerUserID, ok := wukong.ParseIMUID(in.GetChannelId())
		if !ok || peerUserID <= 0 {
			return &impb.MarkConversationReadResponse{
				Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid read request"),
			}, nil
		}
		if err := l.svcCtx.Repo.MarkDirectMessagesRead(
			l.ctx,
			in.GetUserId(),
			peerUserID,
			in.GetMessageSeq(),
		); err != nil {
			return nil, err
		}
	}

	if in.GetMessageSeq() > 0 {
		_ = l.svcCtx.WuKongClient.MarkConversationRead(
			l.ctx,
			wukong.FormatIMUID(in.GetUserId()),
			in.GetChannelId(),
			int32(in.GetConversationType()),
			in.GetMessageSeq(),
		)
	}

	return &impb.MarkConversationReadResponse{
		Base: newStatus(impb.IMCode_IM_CODE_OK, "ok"),
	}, nil
}
