package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/backoffice"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func adminCORSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

type adminLoginReq struct {
	Username       string                `json:"username"`
	Password       string                `json:"password"`
	BehaviorVerify behaviorVerifyRequest `json:"behavior_verify"`
}

type heartbeatReq struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Device   string `json:"device"`
}

type adminUserStatusReq struct {
	UserID int64 `json:"user_id"`
	Status int   `json:"status"`
}

type adminUserRestrictionReq struct {
	UserID        int64    `json:"user_id"`
	Scopes        []string `json:"scopes"`
	DurationValue int64    `json:"duration_value"`
	DurationUnit  string   `json:"duration_unit"`
	Reason        string   `json:"reason"`
}

func adminLoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		var req adminLoginReq
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.PassportRPC.Login(r.Context(), &passportpb.LoginRequest{
			Account:        strings.TrimSpace(req.Username),
			Password:       req.Password,
			BehaviorVerify: toBehaviorVerifyToken(req.BehaviorVerify),
		})
		if err != nil {
			WriteError(w, ErrRPCFailed)
			return
		}
		if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || resp.GetTokens() == nil {
			writePassportStatus(w, resp.GetBase(), nil)
			return
		}

		authResp, err := svcCtx.PassportRPC.GetMyAuthorization(r.Context(), &passportpb.GetMyAuthorizationRequest{
			AccessToken: resp.GetTokens().GetAccessToken(),
		})
		if err != nil {
			WriteError(w, ErrRPCFailed)
			return
		}
		if authResp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || !hasBackofficeAccess(svcCtx, authResp.GetSnapshot()) {
			WriteError(w, ErrPermissionDenied)
			return
		}

		WriteSuccess(w, map[string]any{
			"token":         resp.GetTokens().GetAccessToken(),
			"refresh_token": resp.GetTokens().GetRefreshToken(),
			"user":          userToMap(resp.GetUser()),
			"roles":         roleInfosToMap(authResp.GetSnapshot().GetRoles()),
			"permissions":   permissionInfosToMap(authResp.GetSnapshot().GetPermissions()),
		})
	}
}

func adminLogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		var req logoutRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.TrimSpace(req.RefreshToken) != "" {
			_, _ = svcCtx.PassportRPC.Logout(r.Context(), &passportpb.LogoutRequest{
				RefreshToken: strings.TrimSpace(req.RefreshToken),
			})
		}
		WriteSuccess(w, nil)
	}
}

func adminMeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, _, err := loadAdminAuthorization(r, svcCtx)
		if err != nil || !hasBackofficeAccess(svcCtx, snapshot) {
			WriteError(w, ErrUnauthorized)
			return
		}
		WriteSuccess(w, map[string]any{
			"user":        userToMap(snapshot.GetUser()),
			"roles":       roleInfosToMap(snapshot.GetRoles()),
			"permissions": permissionInfosToMap(snapshot.GetPermissions()),
		})
	}
}

func heartbeatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil || !authResult.Allowed {
			WriteError(w, ErrUnauthorized)
			return
		}
		var req heartbeatReq
		_ = json.NewDecoder(r.Body).Decode(&req)

		err = svcCtx.Backoffice.RecordHeartbeat(r.Context(), backoffice.HeartbeatInput{
			UserID:   authResult.User.GetUserId(),
			Version:  req.Version,
			Platform: req.Platform,
			Device:   req.Device,
		})
		if err != nil {
			logx.Errorf("heartbeat redis: %v", err)
		}
		WriteSuccess(w, nil)
	}
}

func adminStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.read"); !ok {
			return
		}

		stats, err := svcCtx.Backoffice.DashboardStats(r.Context(), time.Now())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"online_count":          stats.OnlineCount,
			"total_users":           stats.TotalUsers,
			"today_new_users":       stats.TodayNewUsers,
			"version_distribution":  distributionItemsToMap(stats.VersionDistribution),
			"platform_distribution": distributionItemsToMap(stats.PlatformDistribution),
			"device_distribution":   distributionItemsToMap(stats.DeviceDistribution),
		})
	}
}

func adminUsersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.read"); !ok {
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		const pageSize = 20

		result, err := svcCtx.Backoffice.ListUsers(r.Context(), backoffice.UserListFilter{
			Page:     page,
			PageSize: pageSize,
			Search:   q.Get("q"),
		})
		if err != nil {
			logx.Errorf("admin users query: %v", err)
			WriteError(w, ErrInternalError)
			return
		}

		type userItem struct {
			ID              int64    `json:"id"`
			Account         string   `json:"account"`
			Nickname        string   `json:"nickname"`
			AvatarURL       string   `json:"avatar_url"`
			AccountStatus   int      `json:"account_status"`
			PostMuteUntil   string   `json:"post_mute_until"`
			DMMuteUntil     string   `json:"dm_mute_until"`
			AccountBanUntil string   `json:"account_ban_until"`
			ViolationCount  int64    `json:"violation_count"`
			CreatedAt       string   `json:"created_at"`
			Roles           []string `json:"roles"`
			RoleNames       []string `json:"role_names"`
		}

		users := make([]userItem, 0, len(result.Users))
		for _, item := range result.Users {
			users = append(users, userItem{
				ID:              item.ID,
				Account:         item.Account,
				Nickname:        item.Nickname,
				AvatarURL:       item.AvatarURL,
				AccountStatus:   item.AccountStatus,
				PostMuteUntil:   formatOptionalTime(item.PostMuteUntil),
				DMMuteUntil:     formatOptionalTime(item.DMMuteUntil),
				AccountBanUntil: formatOptionalTime(item.AccountBanUntil),
				ViolationCount:  item.ViolationCount,
				CreatedAt:       item.CreatedAt.Format("2006-01-02 15:04:05"),
				Roles:           item.Roles,
				RoleNames:       item.RoleNames,
			})
		}

		WriteSuccess(w, map[string]any{
			"users":       users,
			"total":       result.Total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": (result.Total + int64(pageSize) - 1) / int64(pageSize),
		})
	}
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func adminUpdateUserStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.ban"); !ok {
			return
		}
		var req adminUserStatusReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := svcCtx.Backoffice.UpdateUserStatus(r.Context(), req.UserID, req.Status); errors.Is(err, backoffice.ErrInvalidInput) {
			WriteError(w, ErrInvalidArgument)
			return
		} else if err != nil {
			logx.Errorf("update user status: %v", err)
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, nil)
	}
}

func adminApplyUserRestrictionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.ban")
		if !ok {
			return
		}

		var req adminUserRestrictionReq
		if !decodeJSON(w, r, &req) {
			return
		}

		duration, valid := parseAdminDuration(req.DurationValue, req.DurationUnit)
		if !valid {
			WriteError(w, ErrInvalidArgument)
			return
		}
		scopes := normalizeRestrictionScopes(req.Scopes)
		if len(scopes) == 0 {
			WriteError(w, ErrInvalidArgument)
			return
		}
		for _, scope := range scopes {
			err := svcCtx.Backoffice.UpsertUserRestriction(r.Context(), backoffice.UserRestrictionInput{
				UserID:         req.UserID,
				Scope:          scope,
				Reason:         strings.TrimSpace(req.Reason),
				Duration:       duration,
				OperatorUserID: snapshot.GetUser().GetUserId(),
			})
			if errors.Is(err, backoffice.ErrInvalidInput) {
				WriteError(w, ErrInvalidArgument)
				return
			}
			if err != nil {
				logx.Errorf("apply user restriction: %v", err)
				WriteError(w, ErrInternalError)
				return
			}
		}
		WriteSuccess(w, nil)
	}
}

func distributionItemsToMap(items []backoffice.DistributionItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"name": item.Name, "count": item.Count})
	}
	return result
}

func parseAdminDuration(value int64, unit string) (time.Duration, bool) {
	if value <= 0 {
		return 0, false
	}
	switch strings.TrimSpace(unit) {
	case "minute":
		return time.Duration(value) * time.Minute, true
	case "hour":
		return time.Duration(value) * time.Hour, true
	case "day":
		return time.Duration(value) * 24 * time.Hour, true
	case "month":
		return time.Duration(value) * 30 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func normalizeRestrictionScopes(scopes []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		switch scope {
		case backoffice.UserRestrictionScopePostPublish, backoffice.UserRestrictionScopeDirectMessage, backoffice.UserRestrictionScopeAccountLogin:
			if _, ok := seen[scope]; ok {
				continue
			}
			seen[scope] = struct{}{}
			items = append(items, scope)
		}
	}
	return items
}

func permissionInfosToMap(items []*passportpb.PermissionInfo) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"permission_id":   item.GetPermissionId(),
			"permission_code": item.GetPermissionCode(),
			"permission_name": item.GetPermissionName(),
			"module":          item.GetModule(),
			"description":     item.GetDescription(),
		})
	}
	return result
}
