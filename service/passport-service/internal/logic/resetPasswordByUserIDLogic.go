package logic

import (
	"context"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

type ResetPasswordByUserIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetPasswordByUserIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordByUserIDLogic {
	return &ResetPasswordByUserIDLogic{ctx: ctx, svcCtx: svcCtx}
}

// ResetPasswordByUserID 供网关在校园授权校验通过后重置平台密码。
func (l *ResetPasswordByUserIDLogic) ResetPasswordByUserID(in *passportpb.ResetPasswordByUserIDRequest) (*passportpb.ResetPasswordByUserIDResponse, error) {
	if in.GetUserId() <= 0 {
		return &passportpb.ResetPasswordByUserIDResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "user_id must be greater than 0"),
		}, nil
	}

	user, found, err := l.svcCtx.UserStore.GetByID(in.GetUserId())
	if err != nil {
		return &passportpb.ResetPasswordByUserIDResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !found {
		return &passportpb.ResetPasswordByUserIDResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "user not found"),
		}, nil
	}

	code, message, err := updatePlatformPassword(l.svcCtx, user, in.GetNewPassword())
	if err != nil {
		return &passportpb.ResetPasswordByUserIDResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if code != passportpb.PassportCode_PASSPORT_CODE_OK {
		return &passportpb.ResetPasswordByUserIDResponse{
			Base: newStatus(code, message),
		}, nil
	}

	return &passportpb.ResetPasswordByUserIDResponse{
		Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
	}, nil
}
