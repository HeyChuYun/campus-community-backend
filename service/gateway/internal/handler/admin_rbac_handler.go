package handler

import (
	"errors"
	"net/http"

	"hfjj/backend/gateway/internal/backoffice"
	"hfjj/backend/gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type adminCreateRoleReq struct {
	RoleCode string `json:"role_code"`
	RoleName string `json:"role_name"`
}

type adminDeleteRoleReq struct {
	RoleCode string `json:"role_code"`
}

type adminSetRolePermissionsReq struct {
	RoleCode        string   `json:"role_code"`
	PermissionCodes []string `json:"permission_codes"`
}

type adminAssignUserRolesReq struct {
	UserID       int64    `json:"user_id"`
	TargetUserID int64    `json:"target_user_id"`
	RoleCodes    []string `json:"role_codes"`
}

func adminListRolesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.role.assign"); !ok {
			return
		}
		roles, err := svcCtx.Backoffice.RBAC().ListRoles(r.Context())
		if err != nil {
			logx.Errorf("admin list roles: %v", err)
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{"roles": roles})
	}
}

func adminCreateRoleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.role.assign"); !ok {
			return
		}
		var req adminCreateRoleReq
		if !decodeJSON(w, r, &req) {
			return
		}
		roles, err := svcCtx.Backoffice.RBAC().CreateRole(r.Context(), req.RoleCode, req.RoleName)
		if writeRBACError(w, err) {
			return
		}
		WriteSuccess(w, map[string]any{"roles": roles})
	}
}

func adminDeleteRoleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.role.assign"); !ok {
			return
		}
		var req adminDeleteRoleReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := svcCtx.Backoffice.RBAC().DeleteRole(r.Context(), req.RoleCode); writeRBACError(w, err) {
			return
		}
		WriteSuccess(w, nil)
	}
}

func adminListPermissionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.role.assign"); !ok {
			return
		}
		permissions, err := svcCtx.Backoffice.RBAC().ListPermissions(r.Context())
		if err != nil {
			logx.Errorf("admin list permissions: %v", err)
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{"permissions": permissions})
	}
}

func adminSetRolePermissionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.role.assign"); !ok {
			return
		}
		var req adminSetRolePermissionsReq
		if !decodeJSON(w, r, &req) {
			return
		}
		roles, err := svcCtx.Backoffice.RBAC().SetRolePermissions(r.Context(), req.RoleCode, req.PermissionCodes)
		if writeRBACError(w, err) {
			return
		}
		WriteSuccess(w, map[string]any{"roles": roles})
	}
}

func adminAssignUserRolesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.user.role.assign"); !ok {
			return
		}
		var req adminAssignUserRolesReq
		if !decodeJSON(w, r, &req) {
			return
		}
		targetUserID := req.UserID
		if targetUserID <= 0 {
			targetUserID = req.TargetUserID
		}
		result, err := svcCtx.Backoffice.RBAC().AssignUserRoles(r.Context(), targetUserID, req.RoleCodes)
		if writeRBACError(w, err) {
			return
		}
		WriteSuccess(w, map[string]any{
			"user_id":    result.UserID,
			"role_codes": result.RoleCodes,
			"role_names": result.RoleNames,
		})
	}
}

func writeRBACError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, backoffice.ErrInvalidRole), errors.Is(err, backoffice.ErrInvalidInput), errors.Is(err, backoffice.ErrPermissionUnknown):
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: err.Error()})
	case errors.Is(err, backoffice.ErrSystemRole), errors.Is(err, backoffice.ErrRoleExists), errors.Is(err, backoffice.ErrRoleNotFound), errors.Is(err, backoffice.ErrUserNotFound):
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: err.Error()})
	default:
		logx.Errorf("admin rbac: %v", err)
		WriteError(w, ErrInternalError)
	}
	return true
}
