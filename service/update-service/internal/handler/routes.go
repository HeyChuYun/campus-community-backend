package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"hfjj/backend/update-service/internal/appversion"
	"hfjj/backend/update-service/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

type jsonResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

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

func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/app/version", Handler: appVersionHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/app/versions", Handler: adminListAppVersionsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/app/versions", Handler: adminCreateAppVersionHandler(svcCtx)},
	})
}

func appVersionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentVersion := strings.TrimSpace(r.URL.Query().Get("current_version"))
		platform := appversion.NormalizePlatform(r.URL.Query().Get("platform"))

		record, _, err := svcCtx.AppVersion.Latest(r.Context(), platform)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: 500, Message: err.Error()})
			return
		}

		forceUpdate := record.ForceUpdate && appversion.SemverCompare(currentVersion, record.VersionName) < 0
		if record.MinSupportedVersion != "" && appversion.SemverCompare(currentVersion, record.MinSupportedVersion) < 0 {
			forceUpdate = true
		}
		if currentVersion == "" {
			forceUpdate = record.ForceUpdate
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code: 0,
			Data: AppVersionResponse{
				LatestVersion:       record.VersionName,
				VersionCode:         record.VersionCode,
				DownloadURL:         record.DownloadURL,
				AppStoreURL:         record.AppStoreURL,
				ForceUpdate:         forceUpdate,
				MinSupportedVersion: record.MinSupportedVersion,
				UpdateLog:           record.UpdateLog,
			},
		})
	}
}

func adminListAppVersionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := svcCtx.AppVersion.List(r.Context(), r.URL.Query().Get("platform"), 100)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: 500, Message: err.Error()})
			return
		}
		mapped := make([]map[string]any, 0, len(items))
		for _, item := range items {
			mapped = append(mapped, appVersionToMap(item))
		}
		writeJSON(w, http.StatusOK, jsonResponse{Code: 0, Data: map[string]any{"items": mapped}})
	}
}

func adminCreateAppVersionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminCreateAppVersionReq
		if !decodeJSON(w, r, &req) {
			return
		}

		versionName := strings.TrimSpace(req.VersionName)
		if versionName == "" {
			versionName = strings.TrimSpace(req.Version)
		}
		if versionName == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Code: 400, Message: "version_name is required"})
			return
		}
		isActive := true
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		item, err := svcCtx.AppVersion.Upsert(r.Context(), appversion.UpsertInput{
			VersionName:         versionName,
			VersionCode:         req.VersionCode,
			Platform:            req.Platform,
			DownloadURL:         req.DownloadURL,
			AppStoreURL:         req.AppStoreURL,
			ForceUpdate:         req.ForceUpdate,
			MinSupportedVersion: req.MinSupportedVersion,
			UpdateLog:           req.UpdateLog,
			IsActive:            isActive,
			CreatedBy:           "admin",
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: 500, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, jsonResponse{Code: 0, Data: map[string]any{"version": appVersionToMap(item)}})
	}
}

func appVersionToMap(item appversion.Record) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"version_name":          item.VersionName,
		"version_code":          item.VersionCode,
		"platform":              item.Platform,
		"download_url":          item.DownloadURL,
		"app_store_url":         item.AppStoreURL,
		"force_update":          item.ForceUpdate,
		"min_supported_version": item.MinSupportedVersion,
		"update_log":            item.UpdateLog,
		"is_active":             item.IsActive,
		"created_at":            item.CreatedAt.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, payload jsonResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: 400, Message: err.Error()})
		return false
	}
	return true
}
