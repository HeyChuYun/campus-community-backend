package logic

import (
	"context"
	"strings"

	"hfjj/backend/im-service/internal/repository"
	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/im-service/internal/wukong"
	"hfjj/backend/proto/impb"
)

type SendDirectMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendDirectMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendDirectMessageLogic {
	return &SendDirectMessageLogic{ctx: ctx, svcCtx: svcCtx}
}

// SendDirectMessage handles permission checks, quota consumption, WuKongIM send,
// and message audit persistence.
func (l *SendDirectMessageLogic) SendDirectMessage(in *impb.SendDirectMessageRequest) (*impb.SendDirectMessageResponse, error) {
	if in.GetSenderUserId() <= 0 || in.GetReceiverUserId() <= 0 || strings.TrimSpace(in.GetContentText()) == "" {
		return &impb.SendDirectMessageResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "\u6d88\u606f\u8bf7\u6c42\u65e0\u6548"),
		}, nil
	}

	content := strings.TrimSpace(in.GetContentText())
	permission, code, message, err := resolveDirectMessagePermission(l.ctx, l.svcCtx, in.GetSenderUserId(), in.GetReceiverUserId())
	if err != nil {
		return nil, err
	}
	if code != impb.IMCode_IM_CODE_OK || permission == nil || !permission.Allowed {
		return &impb.SendDirectMessageResponse{
			Base:       newStatus(code, message),
			Permission: permission,
		}, nil
	}

	if !permission.GetIsMutualFollow() {
		quota, quotaErr := l.svcCtx.Repo.ConsumeQuota(
			l.ctx,
			in.GetSenderUserId(),
			in.GetReceiverUserId(),
			l.svcCtx.Config.DirectMessage.NonMutualLimit,
		)
		if quotaErr != nil {
			if quotaErr == repository.ErrQuotaExceeded {
				permission.Allowed = false
				permission.RemainingCount = 0
				permission.DenyReason = "\u975e\u4e92\u76f8\u5173\u6ce8\u79c1\u4fe1\u6b21\u6570\u5df2\u7528\u5b8c"
				return &impb.SendDirectMessageResponse{
					Base:       newStatus(impb.IMCode_IM_CODE_DM_LIMIT_REACHED, "\u79c1\u4fe1\u6b21\u6570\u5df2\u7528\u5b8c"),
					Permission: permission,
				}, nil
			}
			return nil, quotaErr
		}
		permission.RemainingCount = quota.LimitCount - quota.UsedCount
	}

	senderIMUID := wukong.FormatIMUID(in.GetSenderUserId())
	receiverIMUID := wukong.FormatIMUID(in.GetReceiverUserId())
	if _, err := l.svcCtx.Repo.UpsertBinding(l.ctx, in.GetSenderUserId(), senderIMUID); err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Repo.UpsertBinding(l.ctx, in.GetReceiverUserId(), receiverIMUID); err != nil {
		return nil, err
	}

	messageID, messageSeq, err := l.svcCtx.WuKongClient.SendDirectMessage(
		l.ctx,
		senderIMUID,
		receiverIMUID,
		in.GetClientMsgNo(),
		content,
		in.GetPayload(),
	)
	if err != nil {
		return &impb.SendDirectMessageResponse{
			Base:       newStatus(impb.IMCode_IM_CODE_UPSTREAM_ERROR, err.Error()),
			Permission: permission,
		}, nil
	}

	_ = l.svcCtx.Repo.CreateMessageAudit(l.ctx, repository.MessageAudit{
		MessageIDInWK:    messageID,
		MessageSeq:       messageSeq,
		ClientMsgNo:      in.GetClientMsgNo(),
		ConversationType: int64(impb.ConversationType_CONVERSATION_TYPE_PERSONAL),
		SenderUserID:     in.GetSenderUserId(),
		ReceiverUserID:   in.GetReceiverUserId(),
		BizType:          int64(impb.MessageBizType_MESSAGE_BIZ_TYPE_DIRECT_MESSAGE),
		ContentText:      content,
		ContentPreview:   sanitizeContentPreview(content),
		Status:           "sent",
	})

	return &impb.SendDirectMessageResponse{
		Base:       newStatus(impb.IMCode_IM_CODE_OK, "\u6210\u529f"),
		Permission: permission,
		Receipt: &impb.DirectMessageReceipt{
			MessageId:        messageID,
			MessageSeq:       messageSeq,
			ClientMsgNo:      in.GetClientMsgNo(),
			ChannelId:        receiverIMUID,
			ConversationType: impb.ConversationType_CONVERSATION_TYPE_PERSONAL,
		},
	}, nil
}
