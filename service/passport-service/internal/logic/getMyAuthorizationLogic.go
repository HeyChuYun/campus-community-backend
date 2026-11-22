package logic

import (
	"context"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMyAuthorizationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyAuthorizationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyAuthorizationLogic {
	return &GetMyAuthorizationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMyAuthorization 返回当前 access_token 对应用户的 Casbin 授权快照。
func (l *GetMyAuthorizationLogic) GetMyAuthorization(in *passportpb.GetMyAuthorizationRequest) (*passportpb.GetMyAuthorizationResponse, error) {
	// 第一步：校验 access_token，并拿到当前平台用户。
	user, code, message, ok, err := loadUserByAccessToken(l.svcCtx, in.GetAccessToken())
	if err != nil {
		return &passportpb.GetMyAuthorizationResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if !ok {
		return &passportpb.GetMyAuthorizationResponse{
			Base: newStatus(code, message),
		}, nil
	}

	// 第二步：读取 Casbin 角色和隐式权限快照。
	snapshot, err := loadAuthorizationSnapshot(l.svcCtx, user)
	if err != nil {
		return &passportpb.GetMyAuthorizationResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.GetMyAuthorizationResponse{
		Base:     newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Snapshot: snapshot,
	}, nil
}
