package handler

import (
	"net/http"
	"strconv"
	"strings"

	"hfjj/backend/gateway/internal/moderation"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type adminBlockPostReq struct {
	PostID              int64    `json:"post_id"`
	Categories          []string `json:"categories"`
	Reason              string   `json:"reason"`
	MuteDurationSeconds int64    `json:"mute_duration_seconds"`
	MuteScope           string   `json:"mute_scope"`
}

type adminRestorePostReq struct {
	PostID int64 `json:"post_id"`
}

func adminListContentPostsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.post.review.read"); !ok {
			return
		}

		page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
		status, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("status")))
		result, err := svcCtx.Moderation.ListPosts(r.Context(), moderation.PostListFilter{
			Page:       page,
			PageSize:   20,
			Query:      r.URL.Query().Get("q"),
			Status:     status,
			CampusCode: r.URL.Query().Get("campus_code"),
		})
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		items := make([]map[string]any, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, map[string]any{
				"post_id":              item.PostID,
				"author_user_id":       item.AuthorUserID,
				"author_account":       item.AuthorAccount,
				"author_nickname":      item.AuthorNickname,
				"content_text":         item.ContentText,
				"status":               item.Status,
				"campus_code":          item.CampusCode,
				"visibility":           item.Visibility,
				"like_count":           item.LikeCount,
				"comment_count":        item.CommentCount,
				"favorite_count":       item.FavoriteCount,
				"created_at":           item.CreatedAt.Format("2006-01-02 15:04:05"),
				"updated_at":           item.UpdatedAt.Format("2006-01-02 15:04:05"),
				"latest_violation_at":  formatOptionalTime(item.LatestViolationAt),
				"latest_reason":        item.LatestReason,
				"latest_source":        item.LatestSource,
				"latest_categories":    item.LatestCategories,
				"post_mute_until":      formatOptionalTime(item.PostMuteUntil),
			})
		}

		WriteSuccess(w, map[string]any{
			"items":       items,
			"total":       result.Total,
			"page":        maxPage(page),
			"page_size":   20,
			"total_pages": (result.Total + 19) / 20,
		})
	}
}

func adminViolationCategoriesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.post.review.read"); !ok {
			return
		}
		WriteSuccess(w, map[string]any{
			"items": svcCtx.Moderation.ViolationCategories(),
		})
	}
}

func adminBlockPostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.post.review.manage")
		if !ok {
			return
		}
		var req adminBlockPostReq
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.ContentRPC.AdminBlockPost(r.Context(), &contentpb.AdminBlockPostRequest{
			OperatorUserId:      snapshot.GetUser().GetUserId(),
			PostId:              req.PostID,
			Categories:          req.Categories,
			Reason:              req.Reason,
			MuteDurationSeconds: req.MuteDurationSeconds,
			MuteScope:           req.MuteScope,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"post": postToMap(svcCtx, resp.GetPost()),
		})
	}
}

func adminRestorePostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.post.review.manage"); !ok {
			return
		}

		var req adminRestorePostReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.PostID <= 0 {
			WriteError(w, ErrInvalidArgument.WithDetail("field", "post_id"))
			return
		}

		restored, err := svcCtx.Moderation.RestorePostVisibility(r.Context(), req.PostID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if !restored {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "帖子当前不是违规封禁状态，无法恢复展示"))
			return
		}

		WriteSuccess(w, map[string]any{
			"post_id": req.PostID,
			"status":  1,
		})
	}
}

func adminUserViolationsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.post.review.read"); !ok {
			return
		}
		userID, ok := parseRequiredQueryInt64(w, r, "user_id")
		if !ok {
			return
		}
		page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
		result, err := svcCtx.Moderation.ListUserViolations(r.Context(), moderation.UserViolationFilter{
			UserID:   userID,
			Page:     page,
			PageSize: 20,
		})
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		items := make([]map[string]any, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, map[string]any{
				"violation_no":          item.ViolationNo,
				"post_id":               item.PostID,
				"source":                item.Source,
				"categories":            item.Categories,
				"reason":                item.Reason,
				"mute_scope":            item.MuteScope,
				"mute_duration_seconds": item.MuteDurationSeconds,
				"operator_user_id":      item.OperatorUserID,
				"created_at":            item.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}

		WriteSuccess(w, map[string]any{
			"items":       items,
			"total":       result.Total,
			"page":        maxPage(page),
			"page_size":   20,
			"total_pages": (result.Total + 19) / 20,
		})
	}
}

func maxPage(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
