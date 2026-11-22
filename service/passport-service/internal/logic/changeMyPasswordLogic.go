package logic

import (
	"context"
	"strings"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

type ChangeMyPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeMyPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeMyPasswordLogic {
	return &ChangeMyPasswordLogic{ctx: ctx, svcCtx: svcCtx}
}

// ChangeMyPassword 修改当前登录用户的平台密码。
func (l *ChangeMyPasswordLogic) ChangeMyPassword(in *passportpb.ChangeMyPasswordRequest) (*passportpb.ChangeMyPasswordResponse, error) {
	operator, code, message, ok, err := loadUserByAccessToken(l.svcCtx, strings.TrimSpace(in.GetAccessToken()))
	if err != nil {
		return &passportpb.ChangeMyPasswordResponse{
			Base: newStatus(code, err.Error()),
		}, nil
	}
	if !ok {
		return &passportpb.ChangeMyPasswordResponse{
			Base: newStatus(code, message),
		}, nil
	}

	user, found, err := l.svcCtx.UserStore.GetByID(operator.ID)
	if err != nil {
		return &passportpb.ChangeMyPasswordResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !found {
		return &passportpb.ChangeMyPasswordResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID, "user not found"),
		}, nil
	}

	code, message, err = updatePlatformPassword(l.svcCtx, user, in.GetNewPassword())
	if err != nil {
		return &passportpb.ChangeMyPasswordResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if code != passportpb.PassportCode_PASSPORT_CODE_OK {
		return &passportpb.ChangeMyPasswordResponse{
			Base: newStatus(code, message),
		}, nil
	}

	return &passportpb.ChangeMyPasswordResponse{
		Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
	}, nil
}
