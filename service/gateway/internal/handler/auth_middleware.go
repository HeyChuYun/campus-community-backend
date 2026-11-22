package handler

import (
	"context"
	"net/http"
	"strings"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

// AuthResult 权限校验结果
type AuthResult struct {
	User     *passportpb.UserProfile
	Token    string
	Allowed  bool
	Snapshot *passportpb.AuthorizationSnapshot
}

// UnifiedAuthMiddleware 统一的权限校验中间件
type UnifiedAuthMiddleware struct {
	svcCtx *svc.ServiceContext
}

func NewUnifiedAuthMiddleware(svcCtx *svc.ServiceContext) *UnifiedAuthMiddleware {
	return &UnifiedAuthMiddleware{svcCtx: svcCtx}
}

// VerifyToken 验证访问令牌
func (m *UnifiedAuthMiddleware) VerifyToken(r *http.Request) (*AuthResult, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}

	resp, err := m.svcCtx.PassportRPC.VerifyAccessToken(r.Context(), &passportpb.VerifyAccessTokenRequest{
		AccessToken: token,
	})
	if err != nil {
		logx.Errorf("VerifyAccessToken RPC error: %v", err)
		return nil, ErrRPCFailed
	}

	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || !resp.GetValid() {
		return &AuthResult{Allowed: false}, ErrUnauthorized
	}

	return &AuthResult{
		User:    resp.GetUser(),
		Token:   token,
		Allowed: true,
	}, nil
}

// Authorize 统一的权限校验
func (m *UnifiedAuthMiddleware) Authorize(r *http.Request, permissionCode string, resourceOwnerUserID int64) (*AuthResult, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}

	resp, err := m.svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: resourceOwnerUserID,
	})
	if err != nil {
		logx.Errorf("Authorize RPC error: %v", err)
		return nil, ErrRPCFailed
	}

	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK {
		return &AuthResult{Allowed: false}, mapPassportError(resp.GetBase())
	}

	if !resp.GetAllowed() {
		return &AuthResult{
			User:     resp.GetSnapshot().GetUser(),
			Token:    token,
			Allowed:  false,
			Snapshot: resp.GetSnapshot(),
		}, ErrPermissionDenied
	}

	if resp.GetSnapshot() == nil || resp.GetSnapshot().GetUser() == nil {
		return nil, ErrInvalidResponse
	}

	return &AuthResult{
		User:     resp.GetSnapshot().GetUser(),
		Token:    token,
		Allowed:  true,
		Snapshot: resp.GetSnapshot(),
	}, nil
}

// AuthorizeAllowDenied 权限校验但允许失败（不直接返回错误）
func (m *UnifiedAuthMiddleware) AuthorizeAllowDenied(r *http.Request, permissionCode string, resourceOwnerUserID int64) (*AuthResult, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}

	resp, err := m.svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: resourceOwnerUserID,
	})
	if err != nil {
		logx.Errorf("Authorize RPC error: %v", err)
		return nil, ErrRPCFailed
	}

	if resp.GetSnapshot() == nil || resp.GetSnapshot().GetUser() == nil {
		return nil, ErrInvalidResponse
	}

	return &AuthResult{
		User:     resp.GetSnapshot().GetUser(),
		Token:    token,
		Allowed:  resp.GetAllowed(),
		Snapshot: resp.GetSnapshot(),
	}, nil
}

// CheckPermission 仅检查权限，不返回用户信息
func (m *UnifiedAuthMiddleware) CheckPermission(r *http.Request, permissionCode string, resourceOwnerUserID int64) (bool, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return false, err
	}

	resp, err := m.svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: resourceOwnerUserID,
	})
	if err != nil {
		logx.Errorf("Authorize RPC error: %v", err)
		return false, ErrRPCFailed
	}

	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK {
		return false, nil
	}

	return resp.GetAllowed(), nil
}

// WithUserContext 将用户信息添加到context
func WithUserContext(ctx context.Context, user *passportpb.UserProfile) context.Context {
	if user == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, "user_id", user.GetUserId())
	ctx = context.WithValue(ctx, "user", user)
	return ctx
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", ErrMissingToken
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}

func mapPassportError(base *passportpb.BaseResp) error {
	if base == nil {
		return ErrInternalError
	}

	switch base.GetCode() {
	case passportpb.PassportCode_PASSPORT_CODE_OK:
		return nil
	case passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT:
		return ErrInvalidArgument
	case passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID:
		return ErrUnauthorized
	case passportpb.PassportCode_PASSPORT_CODE_PERMISSION_DENIED:
		return ErrPermissionDenied
	case passportpb.PassportCode_PASSPORT_CODE_ACCOUNT_UNAVAILABLE:
		return ErrAccountUnavailable
	default:
		return ErrInternalError
	}
}
