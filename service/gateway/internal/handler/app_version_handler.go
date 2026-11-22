package handler

import (
	"net/http"
	"strings"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/gateway/internal/updateclient"
)

type AppVersionResponse struct {
	LatestVersion       string `json:"latest_version"`
	VersionCode         int64  `json:"version_code"`
	DownloadURL         string `json:"download_url"`
	AppStoreURL         string `json:"app_store_url"`
	ForceUpdate         bool   `json:"force_update"`
	MinSupportedVersion string `json:"min_supported_version"`
	UpdateLog           string `json:"update_log"`
}

type adminCreateAppVersionReq struct {
	VersionName         string `json:"version_name"`
	Version             string `json:"version"`
	VersionCode         int64  `json:"version_code"`
	Platform            string `json:"platform"`
	DownloadURL         string `json:"download_url"`
	AppStoreURL         string `json:"app_store_url"`
	ForceUpdate         bool   `json:"force_update"`
	MinSupportedVersion string `json:"min_supported_version"`
	UpdateLog           string `json:"update_log"`
	IsActive            *bool  `json:"is_active"`
}

func appVersionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		record, err := svcCtx.Update.Latest(
			r.Context(),
			strings.TrimSpace(r.URL.Query().Get("current_version")),
			strings.TrimSpace(r.URL.Query().Get("platform")),
		)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		WriteSuccess(w, AppVersionResponse{
			LatestVersion:       firstNonEmpty(record.LatestVersion, record.VersionName),
			VersionCode:         record.VersionCode,
			DownloadURL:         record.DownloadURL,
			AppStoreURL:         record.AppStoreURL,
			ForceUpdate:         record.ForceUpdate,
			MinSupportedVersion: record.MinSupportedVersion,
			UpdateLog:           record.UpdateLog,
		})
	}
}

func adminListAppVersionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.content.moderate"); !ok {
			return
		}

		items, err := svcCtx.Update.List(r.Context(), r.URL.Query().Get("platform"))
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		mapped := make([]map[string]any, 0, len(items))
		for _, item := range items {
			mapped = append(mapped, appVersionToMap(item))
		}
		WriteSuccess(w, map[string]any{"items": mapped})
	}
}

func adminCreateAppVersionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.content.moderate"); !ok {
			return
		}
		var req adminCreateAppVersionReq
		if !decodeJSON(w, r, &req) {
			return
		}

		versionName := strings.TrimSpace(req.VersionName)
		if versionName == "" {
			versionName = strings.TrimSpace(req.Version)
		}
		if versionName == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "version_name is required"})
			return
		}
		isActive := true
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		item, err := svcCtx.Update.Upsert(r.Context(), updateclient.UpsertInput{
			VersionName:         versionName,
			Version:             req.Version,
			VersionCode:         req.VersionCode,
			Platform:            req.Platform,
			DownloadURL:         req.DownloadURL,
			AppStoreURL:         req.AppStoreURL,
			ForceUpdate:         req.ForceUpdate,
			MinSupportedVersion: req.MinSupportedVersion,
			UpdateLog:           req.UpdateLog,
			IsActive:            &isActive,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		WriteSuccess(w, map[string]any{"version": appVersionToMap(item)})
	}
}

func appVersionToMap(item updateclient.Version) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"version_name":          firstNonEmpty(item.VersionName, item.LatestVersion),
		"version_code":          item.VersionCode,
		"platform":              item.Platform,
		"download_url":          item.DownloadURL,
		"app_store_url":         item.AppStoreURL,
		"force_update":          item.ForceUpdate,
		"min_supported_version": item.MinSupportedVersion,
		"update_log":            item.UpdateLog,
		"is_active":             item.IsActive,
		"created_at":            item.CreatedAt,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
