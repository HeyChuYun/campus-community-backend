package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"hfjj/backend/gateway/internal/privacy"
	"hfjj/backend/gateway/internal/svc"
)

type privacyBlacklistUpdateRequest struct {
	TargetUserID     int64    `json:"target_user_id"`
	EnabledScopes    []string `json:"enabled_scopes"`
	BlockDirectDM    *bool    `json:"block_direct_message"`
	BlockPostContent *bool    `json:"block_post_content"`
}

func getPrivacyBlacklistHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		targetUserID, ok := parseRequiredQueryInt64(w, r, "target_user_id")
		if !ok {
			return
		}
		if targetUserID <= 0 || targetUserID == user.GetUserId() {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "目标用户无效"))
			return
		}

		status, err := svcCtx.Privacy.BlacklistStatus(r.Context(), user.GetUserId(), targetUserID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{
			"target_user_id": targetUserID,
			"dm_blocked":     status.DMBlocked,
			"post_blocked":   status.PostBlocked,
		})
	}
}

func updatePrivacyBlacklistHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		var req privacyBlacklistUpdateRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.TargetUserID <= 0 || req.TargetUserID == user.GetUserId() {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "目标用户无效"))
			return
		}

		scopes, err := normalizeRequestedPrivacyScopes(req)
		if err != nil {
			WriteError(w, ErrInvalidArgument.WithDetail("message", err.Error()))
			return
		}
		if err := svcCtx.Privacy.ReplaceBlacklistScopes(r.Context(), user.GetUserId(), req.TargetUserID, scopes); err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		status, err := svcCtx.Privacy.BlacklistStatus(r.Context(), user.GetUserId(), req.TargetUserID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{
			"target_user_id": req.TargetUserID,
			"dm_blocked":     status.DMBlocked,
			"post_blocked":   status.PostBlocked,
		})
	}
}

func normalizeRequestedPrivacyScopes(req privacyBlacklistUpdateRequest) ([]string, error) {
	if req.BlockDirectDM != nil || req.BlockPostContent != nil {
		scopes := make([]string, 0, 2)
		if req.BlockDirectDM != nil && *req.BlockDirectDM {
			scopes = append(scopes, privacy.ScopeDM)
		}
		if req.BlockPostContent != nil && *req.BlockPostContent {
			scopes = append(scopes, privacy.ScopePost)
		}
		return scopes, nil
	}

	scopes := make([]string, 0, len(req.EnabledScopes))
	seen := map[string]struct{}{}
	for _, item := range req.EnabledScopes {
		scope := strings.ToLower(strings.TrimSpace(item))
		if scope == "" {
			continue
		}
		if scope != privacy.ScopeDM && scope != privacy.ScopePost {
			return nil, fmt.Errorf("不支持的黑名单类型: %s", scope)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes, nil
}
