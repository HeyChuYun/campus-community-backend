package logic

import (
	"context"
	"strings"

	"hfjj/backend/im-service/internal/repository"
	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
	"hfjj/backend/proto/relationpb"
)

const (
	accountStatusActive   = 1
	accountStatusReadonly = 2
	accountStatusBanned   = 3
	accountStatusClosed   = 4
)

func resolveDirectMessagePermission(ctx context.Context, svcCtx *svc.ServiceContext, senderUserID, receiverUserID int64) (*impb.DirectMessagePermission, impb.IMCode, string, error) {
	if senderUserID <= 0 || receiverUserID <= 0 {
		return nil, impb.IMCode_IM_CODE_INVALID_ARGUMENT, "用户ID无效", nil
	}

	sender, err := svcCtx.Repo.GetUserBasic(ctx, senderUserID)
	if err != nil {
		return nil, impb.IMCode_IM_CODE_INTERNAL_ERROR, "", err
	}
	if sender.AccountStatus == accountStatusReadonly {
		return &impb.DirectMessagePermission{
			Allowed:    false,
			DenyReason: "当前账号为只读状态，无法发送私信",
		}, impb.IMCode_IM_CODE_READONLY_USER, "只读用户", nil
	}
	if sender.AccountStatus == accountStatusBanned || sender.AccountStatus == accountStatusClosed {
		return &impb.DirectMessagePermission{
			Allowed:    false,
			DenyReason: "当前账号无法发送私信",
		}, impb.IMCode_IM_CODE_PERMISSION_DENIED, "账号不可用", nil
	}

	if senderUserID == receiverUserID {
		return &impb.DirectMessagePermission{
			Allowed:    false,
			DenyReason: "不能给自己发送私信",
		}, impb.IMCode_IM_CODE_PERMISSION_DENIED, "不能给自己发送私信", nil
	}

	relation := relationpb.RelationSnapshot{}
	if svcCtx.RelationRPC != nil {
		resp, rpcErr := svcCtx.RelationRPC.GetRelation(ctx, &relationpb.GetRelationRequest{
			ActorUserId:  senderUserID,
			TargetUserId: receiverUserID,
		})
		if rpcErr != nil {
			return nil, impb.IMCode_IM_CODE_INTERNAL_ERROR, "", rpcErr
		}
		if resp.GetSnapshot() != nil {
			relation = *resp.GetSnapshot()
		}
	}

	permission := &impb.DirectMessagePermission{
		IsMutualFollow: relation.GetIsMutualFollow(),
		IsFollowing:    relation.GetIsFollowing(),
		IsFollowedBy:   relation.GetIsFollowedBy(),
		IsBlocking:     relation.GetIsBlocking(),
		IsBlockedBy:    relation.GetIsBlockedBy(),
	}

	if permission.IsBlocking || permission.IsBlockedBy {
		permission.Allowed = false
		permission.DenyReason = "当前关系不允许发送私信"
		return permission, impb.IMCode_IM_CODE_BLOCKED, "已被拉黑", nil
	}

	if permission.IsMutualFollow {
		permission.Allowed = true
		permission.RemainingCount = -1
		permission.TotalLimit = -1
		return permission, impb.IMCode_IM_CODE_OK, "成功", nil
	}

	quota, quotaErr := svcCtx.Repo.GetQuota(ctx, senderUserID, receiverUserID, svcCtx.Config.DirectMessage.NonMutualLimit)
	if quotaErr != nil {
		return nil, impb.IMCode_IM_CODE_INTERNAL_ERROR, "", quotaErr
	}
	if quota.LimitCount <= 0 {
		quota.LimitCount = svcCtx.Config.DirectMessage.NonMutualLimit
	}

	remaining := quota.LimitCount - quota.UsedCount
	if remaining < 0 {
		remaining = 0
	}

	permission.Allowed = remaining > 0
	permission.RemainingCount = remaining
	permission.TotalLimit = quota.LimitCount
	if !permission.Allowed {
		permission.DenyReason = "非互相关注私信次数已用完"
		return permission, impb.IMCode_IM_CODE_DM_LIMIT_REACHED, "私信次数已用完", nil
	}
	return permission, impb.IMCode_IM_CODE_OK, "成功", nil
}

func makeInteractionPB(item repository.InteractionMessage) *impb.InteractionMessageItem {
	return &impb.InteractionMessageItem{
		InteractionId:     item.ID,
		UserId:            item.UserID,
		EventType:         impb.InteractionEventType(item.EventType),
		OperatorUserId:    item.OperatorUserID,
		OperatorNickname:  item.OperatorNickname,
		OperatorAvatarUrl: item.OperatorAvatarURL,
		Title:             item.Title,
		Content:           item.Content,
		ResourceType:      item.ResourceType,
		ResourceId:        item.ResourceID,
		IsRead:            item.IsRead,
		CreatedAt:         item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func makeSystemNotificationPB(item repository.SystemNotification) *impb.SystemNotificationItem {
	return &impb.SystemNotificationItem{
		NotificationId:   item.ID,
		UserId:           item.UserID,
		NoticeType:       item.NoticeType,
		Title:            item.Title,
		Content:          item.Content,
		ResourceType:     item.ResourceType,
		ResourceId:       item.ResourceID,
		IsRead:           item.IsRead,
		CreatedAt:        item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		MediaPayloadJson: item.MediaPayloadJSON,
	}
}

func makeDirectConversationPB(item repository.DirectConversationSummary) *impb.DirectConversationItem {
	return &impb.DirectConversationItem{
		PeerUserId:              item.PeerUserID,
		PeerAccount:             item.PeerAccount,
		PeerNickname:            item.PeerNickname,
		PeerAvatarUrl:           item.PeerAvatarURL,
		LastMessagePreview:      item.LastMessagePreview,
		LastMessageSenderUserId: item.LastMessageSenderUserID,
		LastMessageCreatedAt:    item.LastMessageCreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UnreadCount:             item.UnreadCount,
	}
}

func makeDirectMessagePB(item repository.DirectMessageRecord) *impb.DirectMessageItem {
	return &impb.DirectMessageItem{
		SenderUserId:   item.SenderUserID,
		ReceiverUserId: item.ReceiverUserID,
		ClientMsgNo:    item.ClientMsgNo,
		ContentText:    item.ContentText,
		MessageId:      item.MessageIDInWK,
		MessageSeq:     item.MessageSeq,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func sanitizeContentPreview(content string) string {
	return repository.TrimPreview(strings.TrimSpace(content))
}
