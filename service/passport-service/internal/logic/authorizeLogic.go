package logic

import (
	"context"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthorizeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthorizeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizeLogic {
	return &AuthorizeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Authorize 统一处理 access_token 校验、账号状态判断和 Casbin 权限判断。
func (l *AuthorizeLogic) Authorize(in *passportpb.AuthorizeRequest) (*passportpb.AuthorizeResponse, error) {
	user, code, message, ok, err := loadUserByAccessToken(l.svcCtx, in.GetAccessToken())
	if err != nil {
		return &passportpb.AuthorizeResponse{
			Base:       newStatus(code, message),
			Allowed:    false,
			DenyReason: message,
		}, nil
	}
	if !ok {
		return &passportpb.AuthorizeResponse{
			Base:       newStatus(code, message),
			Allowed:    false,
			DenyReason: message,
		}, nil
	}

	snapshot, err := loadAuthorizationSnapshot(l.svcCtx, user)
	if err != nil {
		return &passportpb.AuthorizeResponse{
			Base:       newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			Allowed:    false,
			DenyReason: err.Error(),
		}, nil
	}

	// 如果缓存快照里不包含当前要校验的权限点，则立即回源 Casbin 重建一次。
	// 这样新增权限后，老用户不用等 Redis 缓存 TTL 过期也能拿到最新权限。
	if !snapshotContainsPermission(snapshot, in.GetPermissionCode()) {
		snapshot, err = rebuildAuthorizationSnapshot(l.svcCtx, user)
		if err != nil {
			return &passportpb.AuthorizeResponse{
				Base:       newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
				Allowed:    false,
				DenyReason: err.Error(),
			}, nil
		}
	}

	allowed, authCode, denyReason := decideAuthorization(
		l.svcCtx,
		user,
		in.GetPermissionCode(),
		in.GetResourceOwnerUserId(),
	)

	return &passportpb.AuthorizeResponse{
		Base:       newStatus(authCode, denyReason),
		Allowed:    allowed,
		DenyReason: denyReason,
		Snapshot:   snapshot,
	}, nil
}
