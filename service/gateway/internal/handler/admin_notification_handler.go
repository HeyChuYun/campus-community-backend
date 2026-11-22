package handler

import (
	"net/http"
	"strings"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/impb"
)

type adminSendSystemNotificationRequest struct {
	NoticeType       string  `json:"notice_type"`
	Title            string  `json:"title"`
	Content          string  `json:"content"`
	ResourceType     string  `json:"resource_type"`
	ResourceID       int64   `json:"resource_id"`
	MediaPayloadJSON string  `json:"media_payload_json"`
	SendToAll        bool    `json:"send_to_all"`
	TargetUserIDs    []int64 `json:"target_user_ids"`
}

func adminSendSystemNotificationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.notification.send")
		if !ok {
			return
		}

		var req adminSendSystemNotificationRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.ImRPC.CreateSystemNotification(r.Context(), &impb.CreateSystemNotificationRequest{
			OperatorUserId:   snapshot.GetUser().GetUserId(),
			NoticeType:       strings.TrimSpace(req.NoticeType),
			Title:            strings.TrimSpace(req.Title),
			Content:          strings.TrimSpace(req.Content),
			ResourceType:     strings.TrimSpace(req.ResourceType),
			ResourceId:       req.ResourceID,
			MediaPayloadJson: strings.TrimSpace(req.MediaPayloadJSON),
			SendToAll:        req.SendToAll,
			TargetUserIds:    req.TargetUserIDs,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeIMStatus(w, resp.GetBase(), map[string]any{
			"created_count": resp.GetCreatedCount(),
		})
	}
}
