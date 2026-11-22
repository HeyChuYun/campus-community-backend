package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/privacy"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/impb"
)

type imConnectTokenRequest struct {
	DeviceFlag  int32 `json:"device_flag"`
	DeviceLevel int32 `json:"device_level"`
}

type dmCheckRequest struct {
	TargetUserID int64 `json:"target_user_id"`
}

type dmSendRequest struct {
	TargetUserID int64  `json:"target_user_id"`
	ClientMsgNo  string `json:"client_msg_no"`
	ContentText  string `json:"content_text"`
}

type markConversationReadRequest struct {
	ChannelID        string `json:"channel_id"`
	ConversationType int32  `json:"conversation_type"`
	MessageSeq       int64  `json:"message_seq"`
}

type markInteractionReadRequest struct {
	InteractionIDs []int64 `json:"interaction_ids"`
}

type markSystemNotificationReadRequest struct {
	NotificationIDs []int64 `json:"notification_ids"`
}

func imConnectTokenHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req imConnectTokenRequest
		if r.ContentLength > 0 {
			if !decodeJSON(w, r, &req) {
				return
			}
		}
		if req.DeviceFlag == 0 {
			req.DeviceFlag = 1
		}
		if req.DeviceLevel == 0 {
			req.DeviceLevel = 1
		}

		resp, err := svcCtx.ImRPC.IssueConnectToken(r.Context(), &impb.IssueConnectTokenRequest{
			UserId:            user.GetUserId(),
			DeviceId:          strings.TrimSpace(r.Header.Get("X-Device")),
			DeviceFingerprint: strings.TrimSpace(r.Header.Get("X-Device-Fingerprint")),
			DeviceFlag:        req.DeviceFlag,
			DeviceLevel:       req.DeviceLevel,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeIMStatus(w, resp.GetBase(), map[string]any{
			"binding": map[string]any{
				"user_id": resp.GetBinding().GetUserId(),
				"im_uid":  resp.GetBinding().GetImUid(),
				"status":  resp.GetBinding().GetStatus(),
			},
			"connect_token": resp.GetConnectToken(),
			"tcp_addr":      resp.GetTcpAddr(),
			"ws_addr":       resp.GetWsAddr(),
			"wss_addr":      resp.GetWssAddr(),
		})
	}
}

func imListDirectConversationsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		page := int64(1)
		if value := strings.TrimSpace(r.URL.Query().Get("page")); value != "" {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
				page = parsed
			}
		}

		resp, err := svcCtx.ImRPC.ListDirectConversations(r.Context(), &impb.ListDirectConversationsRequest{
			UserId: user.GetUserId(),
			Page:   page,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		items := make([]map[string]any, 0, len(resp.GetItems()))
		for _, item := range resp.GetItems() {
			items = append(items, directConversationToMap(item))
		}

		writeIMStatus(w, resp.GetBase(), map[string]any{
			"items":    items,
			"has_more": resp.GetHasMore(),
		})
	}
}

func imListDirectMessagesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		peerUserID, ok := parseRequiredQueryInt64(w, r, "peer_user_id")
		if !ok {
			return
		}

		page := int64(1)
		if value := strings.TrimSpace(r.URL.Query().Get("page")); value != "" {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
				page = parsed
			}
		}

		resp, err := svcCtx.ImRPC.ListDirectMessages(r.Context(), &impb.ListDirectMessagesRequest{
			UserId:     user.GetUserId(),
			PeerUserId: peerUserID,
			Page:       page,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		items := make([]map[string]any, 0, len(resp.GetItems()))
		for _, item := range resp.GetItems() {
			items = append(items, directMessageToMap(item))
		}

		writeIMStatus(w, resp.GetBase(), map[string]any{
			"items":    items,
			"has_more": resp.GetHasMore(),
		})
	}
}

func imDMCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req dmCheckRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		blocked, err := svcCtx.Privacy.IsScopedBlocked(r.Context(), user.GetUserId(), req.TargetUserID, privacy.ScopeDM)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if blocked {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(impb.IMCode_IM_CODE_BLOCKED),
				Message: "你们之间已开启私信黑名单",
				Data: map[string]any{
					"permission": map[string]any{
						"allowed":          false,
						"is_mutual_follow": false,
						"remaining_count":  0,
						"total_limit":      0,
						"deny_reason":      "你们之间已开启私信黑名单",
					},
				},
			})
			return
		}

		resp, err := svcCtx.ImRPC.CheckDirectMessagePermission(r.Context(), &impb.CheckDirectMessagePermissionRequest{
			SenderUserId:   user.GetUserId(),
			ReceiverUserId: req.TargetUserID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"permission": dmPermissionToMap(resp.GetPermission()),
		})
	}
}

func imDMSendHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req dmSendRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		blocked, err := svcCtx.Privacy.IsScopedBlocked(r.Context(), user.GetUserId(), req.TargetUserID, privacy.ScopeDM)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if blocked {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(impb.IMCode_IM_CODE_BLOCKED),
				Message: "你们之间已开启私信黑名单",
				Data: map[string]any{
					"permission": map[string]any{
						"allowed":          false,
						"is_mutual_follow": false,
						"remaining_count":  0,
						"total_limit":      0,
						"deny_reason":      "你们之间已开启私信黑名单",
					},
					"receipt": map[string]any{},
				},
			})
			return
		}

		resp, err := svcCtx.ImRPC.SendDirectMessage(r.Context(), &impb.SendDirectMessageRequest{
			SenderUserId:   user.GetUserId(),
			ReceiverUserId: req.TargetUserID,
			ClientMsgNo:    req.ClientMsgNo,
			ContentText:    req.ContentText,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		receipt := resp.GetReceipt()
		receiptMap := map[string]any{}
		if receipt != nil {
			receiptMap = map[string]any{
				"message_id":        receipt.GetMessageId(),
				"message_seq":       receipt.GetMessageSeq(),
				"client_msg_no":     receipt.GetClientMsgNo(),
				"channel_id":        receipt.GetChannelId(),
				"conversation_type": int32(receipt.GetConversationType()),
			}
		}
		if resp.GetBase().GetCode() == impb.IMCode_IM_CODE_OK && receipt != nil {
			broadcastDirectMessageEvent(
				svcCtx,
				user.GetUserId(),
				req.TargetUserID,
				req.ClientMsgNo,
				req.ContentText,
				receipt.GetMessageId(),
				receipt.GetMessageSeq(),
				"sent",
				time.Now().Format(time.RFC3339Nano),
			)
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"permission": dmPermissionToMap(resp.GetPermission()),
			"receipt":    receiptMap,
		})
	}
}

func imMarkConversationReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req markConversationReadRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.ImRPC.MarkConversationRead(r.Context(), &impb.MarkConversationReadRequest{
			UserId:           user.GetUserId(),
			ChannelId:        req.ChannelID,
			ConversationType: impb.ConversationType(req.ConversationType),
			MessageSeq:       req.MessageSeq,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if resp.GetBase().GetCode() == impb.IMCode_IM_CODE_OK && req.ConversationType == int32(impb.ConversationType_CONVERSATION_TYPE_PERSONAL) {
			peerUserID := int64(0)
			if strings.HasPrefix(strings.TrimSpace(req.ChannelID), "u_") {
				peerUserID, _ = strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(req.ChannelID), "u_"), 10, 64)
			}
			if peerUserID > 0 {
				event := map[string]any{
					"type":           "conversation_read",
					"reader_user_id": user.GetUserId(),
					"peer_user_id":   peerUserID,
					"message_seq":    req.MessageSeq,
				}
				svcCtx.Realtime.Broadcast(user.GetUserId(), event)
				svcCtx.Realtime.Broadcast(peerUserID, event)
			}
		}
		writeIMStatus(w, resp.GetBase(), nil)
	}
}

func imListInteractionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		page := int64(1)
		if value := strings.TrimSpace(r.URL.Query().Get("page")); value != "" {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
				page = parsed
			}
		}

		resp, err := svcCtx.ImRPC.ListInteractionMessages(r.Context(), &impb.ListInteractionMessagesRequest{
			UserId: user.GetUserId(),
			Page:   page,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		items := make([]map[string]any, 0, len(resp.GetItems()))
		for _, item := range resp.GetItems() {
			items = append(items, interactionItemToMap(item))
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"items":        items,
			"has_more":     resp.GetHasMore(),
			"unread_count": resp.GetUnreadCount(),
		})
	}
}

func imMarkInteractionsReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req markInteractionReadRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.ImRPC.MarkInteractionMessagesRead(r.Context(), &impb.MarkInteractionMessagesReadRequest{
			UserId:         user.GetUserId(),
			InteractionIds: req.InteractionIDs,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"unread_count": resp.GetUnreadCount(),
		})
	}
}

func imListSystemNotificationsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		page := int64(1)
		if value := strings.TrimSpace(r.URL.Query().Get("page")); value != "" {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
				page = parsed
			}
		}

		resp, err := svcCtx.ImRPC.ListSystemNotifications(r.Context(), &impb.ListSystemNotificationsRequest{
			UserId: user.GetUserId(),
			Page:   page,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		items := make([]map[string]any, 0, len(resp.GetItems()))
		for _, item := range resp.GetItems() {
			items = append(items, systemNotificationItemToMap(item))
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"items":        items,
			"has_more":     resp.GetHasMore(),
			"unread_count": resp.GetUnreadCount(),
		})
	}
}

func imMarkSystemNotificationsReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req markSystemNotificationReadRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.ImRPC.MarkSystemNotificationsRead(r.Context(), &impb.MarkSystemNotificationsReadRequest{
			UserId:          user.GetUserId(),
			NotificationIds: req.NotificationIDs,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"unread_count": resp.GetUnreadCount(),
		})
	}
}

func imGetTotalUnreadCountHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		resp, err := svcCtx.ImRPC.GetTotalUnreadCount(r.Context(), &impb.GetTotalUnreadCountRequest{
			UserId: user.GetUserId(),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeIMStatus(w, resp.GetBase(), map[string]any{
			"direct_message_unread":      resp.GetDirectMessageUnread(),
			"interaction_message_unread": resp.GetInteractionMessageUnread(),
			"system_notification_unread": resp.GetSystemNotificationUnread(),
			"total_unread":               resp.GetTotalUnread(),
		})
	}
}

func writeIMStatus(w http.ResponseWriter, base *impb.BaseResp, data any) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    int32(base.GetCode()),
		Message: base.GetMessage(),
		Data:    data,
	})
}

func dmPermissionToMap(permission *impb.DirectMessagePermission) map[string]any {
	if permission == nil {
		return nil
	}
	return map[string]any{
		"allowed":          permission.GetAllowed(),
		"is_mutual_follow": permission.GetIsMutualFollow(),
		"is_following":     permission.GetIsFollowing(),
		"is_followed_by":   permission.GetIsFollowedBy(),
		"is_blocking":      permission.GetIsBlocking(),
		"is_blocked_by":    permission.GetIsBlockedBy(),
		"remaining_count":  permission.GetRemainingCount(),
		"total_limit":      permission.GetTotalLimit(),
		"deny_reason":      permission.GetDenyReason(),
	}
}

func directConversationToMap(item *impb.DirectConversationItem) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"peer_user_id":                item.GetPeerUserId(),
		"peer_account":                item.GetPeerAccount(),
		"peer_nickname":               item.GetPeerNickname(),
		"peer_avatar_url":             item.GetPeerAvatarUrl(),
		"last_message_preview":        item.GetLastMessagePreview(),
		"last_message_sender_user_id": item.GetLastMessageSenderUserId(),
		"last_message_created_at":     item.GetLastMessageCreatedAt(),
		"unread_count":                item.GetUnreadCount(),
	}
}

func directMessageToMap(item *impb.DirectMessageItem) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"sender_user_id":   item.GetSenderUserId(),
		"receiver_user_id": item.GetReceiverUserId(),
		"client_msg_no":    item.GetClientMsgNo(),
		"content_text":     item.GetContentText(),
		"message_id":       item.GetMessageId(),
		"message_seq":      item.GetMessageSeq(),
		"status":           item.GetStatus(),
		"created_at":       item.GetCreatedAt(),
	}
}

func interactionItemToMap(item *impb.InteractionMessageItem) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"interaction_id":      item.GetInteractionId(),
		"user_id":             item.GetUserId(),
		"event_type":          int32(item.GetEventType()),
		"operator_user_id":    item.GetOperatorUserId(),
		"operator_nickname":   item.GetOperatorNickname(),
		"operator_avatar_url": item.GetOperatorAvatarUrl(),
		"title":               item.GetTitle(),
		"content":             item.GetContent(),
		"resource_type":       item.GetResourceType(),
		"resource_id":         item.GetResourceId(),
		"is_read":             item.GetIsRead(),
		"created_at":          item.GetCreatedAt(),
	}
}

func systemNotificationItemToMap(item *impb.SystemNotificationItem) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"notification_id":    item.GetNotificationId(),
		"user_id":            item.GetUserId(),
		"notice_type":        item.GetNoticeType(),
		"title":              item.GetTitle(),
		"content":            item.GetContent(),
		"resource_type":      item.GetResourceType(),
		"resource_id":        item.GetResourceId(),
		"is_read":            item.GetIsRead(),
		"created_at":         item.GetCreatedAt(),
		"media_payload_json": item.GetMediaPayloadJson(),
	}
}
