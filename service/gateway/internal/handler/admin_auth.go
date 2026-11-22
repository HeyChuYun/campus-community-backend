package handler

import (
	"net/http"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/passportpb"
)

func hasBackofficeAccess(svcCtx *svc.ServiceContext, snapshot *passportpb.AuthorizationSnapshot) bool {
	if snapshot == nil || svcCtx == nil || svcCtx.Backoffice == nil {
		return false
	}
	return svcCtx.Backoffice.AllowsAccess(snapshot)
}

func loadAdminAuthorization(r *http.Request, svcCtx *svc.ServiceContext) (*passportpb.AuthorizationSnapshot, string, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, "", err
	}
	resp, err := svcCtx.PassportRPC.GetMyAuthorization(r.Context(), &passportpb.GetMyAuthorizationRequest{
		AccessToken: token,
	})
	if err != nil {
		return nil, "", ErrRPCFailed
	}
	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || resp.GetSnapshot() == nil || resp.GetSnapshot().GetUser() == nil {
		return nil, token, mapPassportError(resp.GetBase())
	}
	return resp.GetSnapshot(), token, nil
}

func verifyAdminToken(svcCtx *svc.ServiceContext, r *http.Request) bool {
	snapshot, _, err := loadAdminAuthorization(r, svcCtx)
	return err == nil && hasBackofficeAccess(svcCtx, snapshot)
}

func authorizeAdminPermission(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, permissionCode string) (*passportpb.AuthorizationSnapshot, bool) {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)
	authResult, err := authMiddleware.Authorize(r, permissionCode, 0)
	if err == nil && authResult != nil && authResult.Allowed && authResult.Snapshot != nil && authResult.Snapshot.GetUser() != nil {
		return authResult.Snapshot, true
	}

	snapshot, _, loadErr := loadAdminAuthorization(r, svcCtx)
	if loadErr != nil || !hasBackofficeAccess(svcCtx, snapshot) {
		WriteError(w, ErrUnauthorized)
		return nil, false
	}
	WriteError(w, ErrPermissionDenied)
	return nil, false
}
