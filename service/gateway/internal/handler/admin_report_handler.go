package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/reporting"
	"hfjj/backend/gateway/internal/svc"
)

type adminReportStatusUpdateReq struct {
	ReportID      int64  `json:"report_id"`
	Status        string `json:"status"`
	DecisionNote  string `json:"decision_note"`
	DecisionCode  string `json:"decision_code"`
	AssignedToUID int64  `json:"assigned_to_user_id"`
}

func adminListReportsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.content.moderate")
		if !ok {
			return
		}

		page := parsePositiveInt(r.URL.Query().Get("page"), 1)
		pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 20)
		if pageSize > 100 {
			pageSize = 100
		}

		result, err := svcCtx.Reporting.AdminList(r.Context(), reporting.AdminListFilter{
			Page:       page,
			PageSize:   pageSize,
			Status:     strings.TrimSpace(r.URL.Query().Get("status")),
			Keyword:    strings.TrimSpace(r.URL.Query().Get("q")),
			TargetType: strings.TrimSpace(r.URL.Query().Get("target_type")),
		})
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		items := make([]map[string]any, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, map[string]any{
				"report_id":            item.ReportID,
				"report_no":            item.ReportNo,
				"reporter_user_id":     item.ReporterUserID,
				"reporter_user":        loadUserSummary(svcCtx, item.ReporterUserID),
				"target_type":          item.TargetType,
				"target_user_id":       item.TargetUserID,
				"target_user":          loadUserSummary(svcCtx, item.TargetUserID),
				"target_post_id":       item.TargetPostID,
				"category_code":        item.CategoryCode,
				"reason_text":          item.ReasonText,
				"status":               item.Status,
				"latest_decision_code": item.LatestDecisionCode,
				"latest_decision_note": item.LatestDecisionNote,
				"assigned_to_user_id":  item.AssignedToUserID,
				"assigned_to_user":     loadUserSummary(svcCtx, item.AssignedToUserID),
				"created_at":           item.CreatedAt.Format(time.RFC3339Nano),
				"updated_at":           item.UpdatedAt.Format(time.RFC3339Nano),
				"viewer_operator_id":   snapshot.GetUser().GetUserId(),
			})
		}

		WriteSuccess(w, map[string]any{
			"items":     items,
			"total":     result.Total,
			"page":      page,
			"page_size": pageSize,
			"total_pages": func() int64 {
				if pageSize <= 0 {
					return 1
				}
				return (result.Total + int64(pageSize) - 1) / int64(pageSize)
			}(),
		})
	}
}

func adminReportDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.content.moderate"); !ok {
			return
		}

		reportID, ok := parseRequiredQueryInt64(w, r, "report_id")
		if !ok {
			return
		}

		detail, exists, err := svcCtx.Reporting.AdminDetail(r.Context(), reportID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if !exists {
			WriteError(w, ErrNotFound)
			return
		}

		evidences := make([]map[string]any, 0, len(detail.Evidences))
		for _, item := range detail.Evidences {
			evidences = append(evidences, map[string]any{
				"id":         item.ID,
				"media_type": item.MediaType,
				"file_url":   item.FileURL,
				"file_size":  item.FileSize,
				"width":      item.Width,
				"height":     item.Height,
				"sort_order": item.SortOrder,
				"created_at": item.CreatedAt.Format(time.RFC3339Nano),
			})
		}

		actions := make([]map[string]any, 0, len(detail.Actions))
		for _, item := range detail.Actions {
			var extra any
			if len(item.Extra) > 0 {
				_ = json.Unmarshal(item.Extra, &extra)
			}
			actions = append(actions, map[string]any{
				"id":               item.ID,
				"operator_user_id": item.OperatorUserID,
				"operator_user":    loadUserSummary(svcCtx, item.OperatorUserID),
				"action_type":      item.ActionType,
				"from_status":      item.FromStatus,
				"to_status":        item.ToStatus,
				"note":             item.Note,
				"extra":            extra,
				"created_at":       item.CreatedAt.Format(time.RFC3339Nano),
			})
		}

		var targetSnapshot any
		if len(detail.Report.TargetSnapshot) > 0 {
			_ = json.Unmarshal(detail.Report.TargetSnapshot, &targetSnapshot)
		}

		WriteSuccess(w, map[string]any{
			"report": map[string]any{
				"report_id":            detail.Report.ReportID,
				"report_no":            detail.Report.ReportNo,
				"reporter_user_id":     detail.Report.ReporterUserID,
				"reporter_user":        loadUserSummary(svcCtx, detail.Report.ReporterUserID),
				"target_type":          detail.Report.TargetType,
				"target_user_id":       detail.Report.TargetUserID,
				"target_user":          loadUserSummary(svcCtx, detail.Report.TargetUserID),
				"target_post_id":       detail.Report.TargetPostID,
				"category_code":        detail.Report.CategoryCode,
				"reason_text":          detail.Report.ReasonText,
				"status":               detail.Report.Status,
				"target_snapshot":      targetSnapshot,
				"latest_decision_code": detail.Report.LatestDecisionCode,
				"latest_decision_note": detail.Report.LatestDecisionNote,
				"assigned_to_user_id":  detail.Report.AssignedToUserID,
				"assigned_to_user":     loadUserSummary(svcCtx, detail.Report.AssignedToUserID),
				"closed_at":            nullableTimePtrToRFC3339(detail.Report.ClosedAt),
				"created_at":           detail.Report.CreatedAt.Format(time.RFC3339Nano),
				"updated_at":           detail.Report.UpdatedAt.Format(time.RFC3339Nano),
			},
			"evidences": evidences,
			"actions":   actions,
		})
	}
}

func adminUpdateReportStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.content.moderate")
		if !ok {
			return
		}

		var req adminReportStatusUpdateReq
		if !decodeJSON(w, r, &req) {
			return
		}
		req.Status = strings.TrimSpace(req.Status)
		if req.ReportID <= 0 || req.Status == "" {
			WriteError(w, ErrInvalidArgument)
			return
		}

		err := svcCtx.Reporting.UpdateAdminStatus(r.Context(), reporting.AdminStatusUpdate{
			ReportID:       req.ReportID,
			Status:         req.Status,
			DecisionNote:   req.DecisionNote,
			DecisionCode:   req.DecisionCode,
			AssignedToUID:  req.AssignedToUID,
			OperatorUserID: snapshot.GetUser().GetUserId(),
		})
		if reporting.IsNotFound(err) {
			WriteError(w, ErrNotFound)
			return
		}
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, nil)
	}
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func nullableTimePtrToRFC3339(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339Nano)
}
