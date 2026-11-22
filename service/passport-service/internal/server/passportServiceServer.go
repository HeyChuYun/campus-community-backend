package server

import (
	"context"

	"hfjj/backend/passport-service/internal/logic"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

type PassportServiceServer struct {
	svcCtx *svc.ServiceContext
	passportpb.UnimplementedPassportServiceServer
}

func NewPassportServiceServer(svcCtx *svc.ServiceContext) *PassportServiceServer {
	return &PassportServiceServer{svcCtx: svcCtx}
}

func (s *PassportServiceServer) Register(ctx context.Context, in *passportpb.RegisterRequest) (*passportpb.RegisterResponse, error) {
	l := logic.NewRegisterLogic(ctx, s.svcCtx)
	return l.Register(in)
}

func (s *PassportServiceServer) Login(ctx context.Context, in *passportpb.LoginRequest) (*passportpb.LoginResponse, error) {
	l := logic.NewLoginLogic(ctx, s.svcCtx)
	return l.Login(in)
}

func (s *PassportServiceServer) GetBehaviorVerifyConfig(ctx context.Context, in *passportpb.GetBehaviorVerifyConfigRequest) (*passportpb.GetBehaviorVerifyConfigResponse, error) {
	l := logic.NewGetBehaviorVerifyConfigLogic(ctx, s.svcCtx)
	return l.GetBehaviorVerifyConfig(in)
}

func (s *PassportServiceServer) RefreshToken(ctx context.Context, in *passportpb.RefreshTokenRequest) (*passportpb.RefreshTokenResponse, error) {
	l := logic.NewRefreshTokenLogic(ctx, s.svcCtx)
	return l.RefreshToken(in)
}

func (s *PassportServiceServer) VerifyAccessToken(ctx context.Context, in *passportpb.VerifyAccessTokenRequest) (*passportpb.VerifyAccessTokenResponse, error) {
	l := logic.NewVerifyAccessTokenLogic(ctx, s.svcCtx)
	return l.VerifyAccessToken(in)
}

func (s *PassportServiceServer) Authorize(ctx context.Context, in *passportpb.AuthorizeRequest) (*passportpb.AuthorizeResponse, error) {
	l := logic.NewAuthorizeLogic(ctx, s.svcCtx)
	return l.Authorize(in)
}

func (s *PassportServiceServer) GetMyAuthorization(ctx context.Context, in *passportpb.GetMyAuthorizationRequest) (*passportpb.GetMyAuthorizationResponse, error) {
	l := logic.NewGetMyAuthorizationLogic(ctx, s.svcCtx)
	return l.GetMyAuthorization(in)
}

func (s *PassportServiceServer) AssignUserRoles(ctx context.Context, in *passportpb.AssignUserRolesRequest) (*passportpb.AssignUserRolesResponse, error) {
	l := logic.NewAssignUserRolesLogic(ctx, s.svcCtx)
	return l.AssignUserRoles(in)
}

func (s *PassportServiceServer) ListUserRoles(ctx context.Context, in *passportpb.ListUserRolesRequest) (*passportpb.ListUserRolesResponse, error) {
	l := logic.NewListUserRolesLogic(ctx, s.svcCtx)
	return l.ListUserRoles(in)
}

func (s *PassportServiceServer) ListRoles(ctx context.Context, in *passportpb.ListRolesRequest) (*passportpb.ListRolesResponse, error) {
	l := logic.NewListRolesLogic(ctx, s.svcCtx)
	return l.ListRoles(in)
}

func (s *PassportServiceServer) ListPermissions(ctx context.Context, in *passportpb.ListPermissionsRequest) (*passportpb.ListPermissionsResponse, error) {
	l := logic.NewListPermissionsLogic(ctx, s.svcCtx)
	return l.ListPermissions(in)
}

func (s *PassportServiceServer) UpdateMyProfile(ctx context.Context, in *passportpb.UpdateMyProfileRequest) (*passportpb.UpdateMyProfileResponse, error) {
	l := logic.NewUpdateMyProfileLogic(ctx, s.svcCtx)
	return l.UpdateMyProfile(in)
}

func (s *PassportServiceServer) ChangeMyPassword(ctx context.Context, in *passportpb.ChangeMyPasswordRequest) (*passportpb.ChangeMyPasswordResponse, error) {
	l := logic.NewChangeMyPasswordLogic(ctx, s.svcCtx)
	return l.ChangeMyPassword(in)
}

func (s *PassportServiceServer) ResetPasswordByUserID(ctx context.Context, in *passportpb.ResetPasswordByUserIDRequest) (*passportpb.ResetPasswordByUserIDResponse, error) {
	l := logic.NewResetPasswordByUserIDLogic(ctx, s.svcCtx)
	return l.ResetPasswordByUserID(in)
}

func (s *PassportServiceServer) Logout(ctx context.Context, in *passportpb.LogoutRequest) (*passportpb.LogoutResponse, error) {
	l := logic.NewLogoutLogic(ctx, s.svcCtx)
	return l.Logout(in)
}
