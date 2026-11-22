package logic

import (
	"context"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyAccessTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyAccessTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyAccessTokenLogic {
	return &VerifyAccessTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyAccessTokenLogic) VerifyAccessToken(in *passportpb.VerifyAccessTokenRequest) (*passportpb.VerifyAccessTokenResponse, error) {
	user, code, message, ok, err := loadUserByAccessToken(l.svcCtx, in.GetAccessToken())
	if err != nil {
		return &passportpb.VerifyAccessTokenResponse{
			Base:      newStatus(code, message),
			Valid:     false,
			TokenType: passportpb.TokenType_TOKEN_TYPE_ACCESS,
		}, nil
	}
	if !ok {
		return &passportpb.VerifyAccessTokenResponse{
			Base:      newStatus(code, message),
			Valid:     false,
			TokenType: passportpb.TokenType_TOKEN_TYPE_ACCESS,
		}, nil
	}
	if user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_BLOCKED) ||
		user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_DEACTIVATED) {
		return &passportpb.VerifyAccessTokenResponse{
			Base:      newStatus(passportpb.PassportCode_PASSPORT_CODE_ACCOUNT_UNAVAILABLE, "账号不可用"),
			Valid:     false,
			TokenType: passportpb.TokenType_TOKEN_TYPE_ACCESS,
		}, nil
	}

	return &passportpb.VerifyAccessTokenResponse{
		Base:      newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Valid:     true,
		User:      buildUserProfile(user),
		TokenType: passportpb.TokenType_TOKEN_TYPE_ACCESS,
	}, nil
}
