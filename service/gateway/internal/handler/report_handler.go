package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"hfjj/backend/gateway/internal/reporting"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/pkg/constants"
)

type reportEvidenceRequest struct {
	FileURL  string `json:"file_url"`
	FileSize int64  `json:"file_size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type reportUserRequest struct {
	TargetUserID int64                   `json:"target_user_id"`
	CategoryCode string                  `json:"category_code"`
	ReasonText   string                  `json:"reason_text"`
	Evidences    []reportEvidenceRequest `json:"evidences"`
}

type reportPostAppealRequest struct {
	PostID     int64                   `json:"post_id"`
	ReasonText string                  `json:"reason_text"`
	Evidences  []reportEvidenceRequest `json:"evidences"`
}

func reportSettingsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil || authResult == nil || !authResult.Allowed {
			WriteError(w, ErrUnauthorized)
			return
		}

		settings, err := svcCtx.Reporting.Settings(r.Context())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"daily_target_limit":        settings.DailyTargetLimit,
			"same_target_cooldown_days": settings.SameTargetCooldown,
			"min_reason_length":         settings.MinReasonLength,
			"max_reason_length":         settings.MaxReasonLength,
			"max_evidence_count":        settings.MaxEvidenceCount,
			"max_evidence_size_bytes":   settings.MaxEvidenceSizeByte,
			"categories":                svcCtx.Reporting.Categories(),
		})
	}
}

func uploadReportEvidenceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil || authResult == nil || !authResult.Allowed || authResult.User == nil {
			WriteError(w, ErrUnauthorized)
			return
		}

		settings, err := svcCtx.Reporting.Settings(r.Context())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		maxBytes := settings.MaxEvidenceSizeByte
		if maxBytes <= 0 {
			maxBytes = 10 << 20
		}

		file, header, contentType, ext, ok := parseUploadFile(w, r, maxBytes)
		if !ok {
			return
		}
		defer file.Close()
		if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "only image upload is allowed"))
			return
		}
		if header.Size <= 0 || header.Size > settings.MaxEvidenceSizeByte {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "evidence image size exceeds limit"))
			return
		}

		key, publicURL, err := uploadToMinIO(r.Context(), svcCtx.Config.MediaStorage, file, header.Size, contentType, ext)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}

		WriteSuccess(w, map[string]any{
			"storage_key":  key,
			"url":          publicURL,
			"size_bytes":   header.Size,
			"content_type": contentType,
		})
	}
}

func submitUserReportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil || authResult == nil || !authResult.Allowed || authResult.User == nil {
			WriteError(w, ErrUnauthorized)
			return
		}

		var req reportUserRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		settings, err := svcCtx.Reporting.Settings(r.Context())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		reporterUserID := authResult.User.GetUserId()
		targetUserID := req.TargetUserID
		categoryCode := strings.TrimSpace(req.CategoryCode)
		reasonText := strings.TrimSpace(req.ReasonText)
		reasonLength := utf8.RuneCountInString(reasonText)

		if reporterUserID <= 0 || targetUserID <= 0 || categoryCode == "" {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "target_user_id/category_code"))
			return
		}
		if reporterUserID == targetUserID {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "target_user_id").WithDetail("message", "不能举报自己"))
			return
		}
		if !svcCtx.Reporting.IsAllowedCategory(categoryCode) {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "category_code").WithDetail("message", "举报分类无效"))
			return
		}
		if reasonLength < settings.MinReasonLength {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "reason_text").WithDetail("message", fmt.Sprintf("举报理由不能少于 %d 个字", settings.MinReasonLength)))
			return
		}
		if reasonLength > settings.MaxReasonLength {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "reason_text").WithDetail("message", fmt.Sprintf("举报理由不能超过 %d 个字", settings.MaxReasonLength)))
			return
		}
		if len(req.Evidences) > settings.MaxEvidenceCount {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "evidences").WithDetail("message", fmt.Sprintf("证据图片不能超过 %d 张", settings.MaxEvidenceCount)))
			return
		}

		evidences := make([]reporting.EvidenceInput, 0, len(req.Evidences))
		for index, item := range req.Evidences {
			if strings.TrimSpace(item.FileURL) == "" {
				WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", fmt.Sprintf("evidences[%d].file_url", index)))
				return
			}
			if item.FileSize <= 0 || item.FileSize > settings.MaxEvidenceSizeByte {
				WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", fmt.Sprintf("evidences[%d].file_size", index)).WithDetail("message", "证据图片大小超出限制"))
				return
			}
			evidences = append(evidences, reporting.EvidenceInput{
				FileURL:  strings.TrimSpace(item.FileURL),
				FileSize: item.FileSize,
				Width:    item.Width,
				Height:   item.Height,
			})
		}

		targetSnapshot, err := svcCtx.Reporting.TargetUserSnapshot(r.Context(), targetUserID)
		if reporting.IsNotFound(err) {
			WriteError(w, ErrNotFound)
			return
		}
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		blockReason, err := svcCtx.Reporting.CanSubmitUserReport(r.Context(), reporterUserID, targetUserID, settings)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if blockReason != "" {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("message", blockReason))
			return
		}

		result, err := svcCtx.Reporting.CreateUserReport(r.Context(), reporting.CreateUserReportInput{
			ReporterUserID: reporterUserID,
			TargetUserID:   targetUserID,
			CategoryCode:   categoryCode,
			ReasonText:     reasonText,
			Evidences:      evidences,
			TargetSnapshot: targetSnapshot,
		})
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"report_id": result.ReportID,
			"report_no": result.ReportNo,
			"status":    result.Status,
		})
	}
}

func reportPostAppealContextHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil || authResult == nil || !authResult.Allowed || authResult.User == nil {
			WriteError(w, ErrUnauthorized)
			return
		}

		postID, ok := parseRequiredQueryInt64(w, r, "post_id")
		if !ok {
			return
		}

		contextData, err := svcCtx.Reporting.PostAppealContext(r.Context(), authResult.User.GetUserId(), postID)
		if reporting.IsNotFound(err) {
			WriteError(w, ErrNotFound)
			return
		}
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"post_id":                 contextData.PostID,
			"author_user_id":          contextData.AuthorUserID,
			"content_text":            contextData.ContentText,
			"post_status":             contextData.PostStatus,
			"latest_violation_no":     contextData.LatestViolationNo,
			"latest_violation_at":     nullableTimePtrToRFC3339(contextData.LatestViolationAt),
			"latest_violation_source": contextData.LatestViolationSource,
			"latest_reason":           contextData.LatestReason,
			"latest_categories":       contextData.LatestCategories,
			"mute_scope":              contextData.MuteScope,
			"mute_duration_seconds":   contextData.MuteDurationSeconds,
			"post_mute_until":         nullableTimePtrToRFC3339(contextData.PostMuteUntil),
			"has_appeal":              contextData.HasAppeal,
			"latest_appeal_status":    contextData.LatestAppealStatus,
			"latest_appeal_at":        nullableTimePtrToRFC3339(contextData.LatestAppealAt),
			"can_submit_appeal":       contextData.CanSubmitAppeal,
			"block_submit_reason":     contextData.BlockSubmitReason,
		})
	}
}

func submitPostAppealHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil || authResult == nil || !authResult.Allowed || authResult.User == nil {
			WriteError(w, ErrUnauthorized)
			return
		}

		var req reportPostAppealRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		settings, err := svcCtx.Reporting.Settings(r.Context())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		reporterUserID := authResult.User.GetUserId()
		postID := req.PostID
		reasonText := strings.TrimSpace(req.ReasonText)
		reasonLength := utf8.RuneCountInString(reasonText)

		if reporterUserID <= 0 || postID <= 0 {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "post_id"))
			return
		}
		if reasonLength < settings.MinReasonLength {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "reason_text").WithDetail("message", fmt.Sprintf("申诉说明不能少于 %d 个字", settings.MinReasonLength)))
			return
		}
		if reasonLength > settings.MaxReasonLength {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "reason_text").WithDetail("message", fmt.Sprintf("申诉说明不能超过 %d 个字", settings.MaxReasonLength)))
			return
		}
		if len(req.Evidences) > settings.MaxEvidenceCount {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", "evidences").WithDetail("message", fmt.Sprintf("补充截图不能超过 %d 张", settings.MaxEvidenceCount)))
			return
		}

		contextData, err := svcCtx.Reporting.PostAppealContext(r.Context(), reporterUserID, postID)
		if reporting.IsNotFound(err) {
			WriteError(w, ErrNotFound)
			return
		}
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		blockReason, err := svcCtx.Reporting.CanSubmitPostAppeal(r.Context(), reporterUserID, postID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if blockReason != "" {
			WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("message", blockReason))
			return
		}

		evidences := make([]reporting.EvidenceInput, 0, len(req.Evidences))
		for index, item := range req.Evidences {
			if strings.TrimSpace(item.FileURL) == "" {
				WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", fmt.Sprintf("evidences[%d].file_url", index)))
				return
			}
			if item.FileSize <= 0 || item.FileSize > settings.MaxEvidenceSizeByte {
				WriteError(w, NewAPIError(constants.CodeInvalidArgument, "请求参数错误").WithDetail("field", fmt.Sprintf("evidences[%d].file_size", index)).WithDetail("message", "证据图片大小超出限制"))
				return
			}
			evidences = append(evidences, reporting.EvidenceInput{
				FileURL:  strings.TrimSpace(item.FileURL),
				FileSize: item.FileSize,
				Width:    item.Width,
				Height:   item.Height,
			})
		}

		targetSnapshot := map[string]any{
			"post_id":                 contextData.PostID,
			"author_user_id":          contextData.AuthorUserID,
			"content_text":            contextData.ContentText,
			"post_status":             contextData.PostStatus,
			"latest_violation_no":     contextData.LatestViolationNo,
			"latest_violation_source": contextData.LatestViolationSource,
			"latest_reason":           contextData.LatestReason,
			"latest_categories":       contextData.LatestCategories,
			"mute_scope":              contextData.MuteScope,
			"mute_duration_seconds":   contextData.MuteDurationSeconds,
			"post_mute_until":         nullableTimePtrToRFC3339(contextData.PostMuteUntil),
		}
		if contextData.LatestViolationAt != nil {
			targetSnapshot["latest_violation_at"] = contextData.LatestViolationAt.Format(time.RFC3339Nano)
		}

		result, err := svcCtx.Reporting.CreatePostAppeal(r.Context(), reporting.CreatePostAppealInput{
			ReporterUserID: reporterUserID,
			TargetUserID:   contextData.AuthorUserID,
			TargetPostID:   contextData.PostID,
			ReasonText:     reasonText,
			Evidences:      evidences,
			TargetSnapshot: targetSnapshot,
		})
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"report_id": result.ReportID,
			"report_no": result.ReportNo,
			"status":    result.Status,
		})
	}
}

func reportMessageForCode(code int32) string {
	switch code {
	case constants.CodeInvalidArgument:
		return "请求参数错误"
	case constants.CodeUnauthorized:
		return "登录状态已失效"
	default:
		return "提交举报失败"
	}
}
