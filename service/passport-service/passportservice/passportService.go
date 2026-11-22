package passportservice

import (
	"context"

	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type (
	AssignUserRolesRequest          = passportpb.AssignUserRolesRequest
	AssignUserRolesResponse         = passportpb.AssignUserRolesResponse
	AuthorizationSnapshot           = passportpb.AuthorizationSnapshot
	AuthorizeRequest                = passportpb.AuthorizeRequest
	AuthorizeResponse               = passportpb.AuthorizeResponse
	BaseResp                        = passportpb.BaseResp
	BehaviorVerifyConfig            = passportpb.BehaviorVerifyConfig
	BehaviorVerifyToken             = passportpb.BehaviorVerifyToken
	ChangeMyPasswordRequest         = passportpb.ChangeMyPasswordRequest
	ChangeMyPasswordResponse        = passportpb.ChangeMyPasswordResponse
	GetBehaviorVerifyConfigRequest  = passportpb.GetBehaviorVerifyConfigRequest
	GetBehaviorVerifyConfigResponse = passportpb.GetBehaviorVerifyConfigResponse
	GetMyAuthorizationRequest       = passportpb.GetMyAuthorizationRequest
	GetMyAuthorizationResponse      = passportpb.GetMyAuthorizationResponse
	LoginRequest                    = passportpb.LoginRequest
	LoginResponse                   = passportpb.LoginResponse
	ListPermissionsRequest          = passportpb.ListPermissionsRequest
	ListPermissionsResponse         = passportpb.ListPermissionsResponse
	ListRolesRequest                = passportpb.ListRolesRequest
	ListRolesResponse               = passportpb.ListRolesResponse
	ListUserRolesRequest            = passportpb.ListUserRolesRequest
	ListUserRolesResponse           = passportpb.ListUserRolesResponse
	LogoutRequest                   = passportpb.LogoutRequest
	LogoutResponse                  = passportpb.LogoutResponse
	PermissionInfo                  = passportpb.PermissionInfo
	RefreshTokenRequest             = passportpb.RefreshTokenRequest
	RefreshTokenResponse            = passportpb.RefreshTokenResponse
	RegisterRequest                 = passportpb.RegisterRequest
	RegisterResponse                = passportpb.RegisterResponse
	ResetPasswordByUserIDRequest    = passportpb.ResetPasswordByUserIDRequest
	ResetPasswordByUserIDResponse   = passportpb.ResetPasswordByUserIDResponse
	RoleInfo                        = passportpb.RoleInfo
	TokenPair                       = passportpb.TokenPair
	UpdateMyProfileRequest          = passportpb.UpdateMyProfileRequest
	UpdateMyProfileResponse         = passportpb.UpdateMyProfileResponse
	UserProfile                     = passportpb.UserProfile
	VerifyAccessTokenRequest        = passportpb.VerifyAccessTokenRequest
	VerifyAccessTokenResponse       = passportpb.VerifyAccessTokenResponse

	PassportService interface {
		Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
		Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error)
		GetBehaviorVerifyConfig(ctx context.Context, in *GetBehaviorVerifyConfigRequest, opts ...grpc.CallOption) (*GetBehaviorVerifyConfigResponse, error)
		RefreshToken(ctx context.Context, in *RefreshTokenRequest, opts ...grpc.CallOption) (*RefreshTokenResponse, error)
		VerifyAccessToken(ctx context.Context, in *VerifyAccessTokenRequest, opts ...grpc.CallOption) (*VerifyAccessTokenResponse, error)
		Authorize(ctx context.Context, in *AuthorizeRequest, opts ...grpc.CallOption) (*AuthorizeResponse, error)
		GetMyAuthorization(ctx context.Context, in *GetMyAuthorizationRequest, opts ...grpc.CallOption) (*GetMyAuthorizationResponse, error)
		AssignUserRoles(ctx context.Context, in *AssignUserRolesRequest, opts ...grpc.CallOption) (*AssignUserRolesResponse, error)
		ListUserRoles(ctx context.Context, in *ListUserRolesRequest, opts ...grpc.CallOption) (*ListUserRolesResponse, error)
		ListRoles(ctx context.Context, in *ListRolesRequest, opts ...grpc.CallOption) (*ListRolesResponse, error)
		ListPermissions(ctx context.Context, in *ListPermissionsRequest, opts ...grpc.CallOption) (*ListPermissionsResponse, error)
		UpdateMyProfile(ctx context.Context, in *UpdateMyProfileRequest, opts ...grpc.CallOption) (*UpdateMyProfileResponse, error)
		ChangeMyPassword(ctx context.Context, in *ChangeMyPasswordRequest, opts ...grpc.CallOption) (*ChangeMyPasswordResponse, error)
		ResetPasswordByUserID(ctx context.Context, in *ResetPasswordByUserIDRequest, opts ...grpc.CallOption) (*ResetPasswordByUserIDResponse, error)
		Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutResponse, error)
	}

	defaultPassportService struct {
		cli zrpc.Client
	}
)

func NewPassportService(cli zrpc.Client) PassportService {
	return &defaultPassportService{cli: cli}
}

func (m *defaultPassportService) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.Register(ctx, in, opts...)
}

func (m *defaultPassportService) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.Login(ctx, in, opts...)
}

func (m *defaultPassportService) GetBehaviorVerifyConfig(ctx context.Context, in *GetBehaviorVerifyConfigRequest, opts ...grpc.CallOption) (*GetBehaviorVerifyConfigResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.GetBehaviorVerifyConfig(ctx, in, opts...)
}

func (m *defaultPassportService) RefreshToken(ctx context.Context, in *RefreshTokenRequest, opts ...grpc.CallOption) (*RefreshTokenResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.RefreshToken(ctx, in, opts...)
}

func (m *defaultPassportService) VerifyAccessToken(ctx context.Context, in *VerifyAccessTokenRequest, opts ...grpc.CallOption) (*VerifyAccessTokenResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.VerifyAccessToken(ctx, in, opts...)
}

func (m *defaultPassportService) Authorize(ctx context.Context, in *AuthorizeRequest, opts ...grpc.CallOption) (*AuthorizeResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.Authorize(ctx, in, opts...)
}

func (m *defaultPassportService) GetMyAuthorization(ctx context.Context, in *GetMyAuthorizationRequest, opts ...grpc.CallOption) (*GetMyAuthorizationResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.GetMyAuthorization(ctx, in, opts...)
}

func (m *defaultPassportService) AssignUserRoles(ctx context.Context, in *AssignUserRolesRequest, opts ...grpc.CallOption) (*AssignUserRolesResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.AssignUserRoles(ctx, in, opts...)
}

func (m *defaultPassportService) ListUserRoles(ctx context.Context, in *ListUserRolesRequest, opts ...grpc.CallOption) (*ListUserRolesResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.ListUserRoles(ctx, in, opts...)
}

func (m *defaultPassportService) ListRoles(ctx context.Context, in *ListRolesRequest, opts ...grpc.CallOption) (*ListRolesResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.ListRoles(ctx, in, opts...)
}

func (m *defaultPassportService) ListPermissions(ctx context.Context, in *ListPermissionsRequest, opts ...grpc.CallOption) (*ListPermissionsResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.ListPermissions(ctx, in, opts...)
}

func (m *defaultPassportService) UpdateMyProfile(ctx context.Context, in *UpdateMyProfileRequest, opts ...grpc.CallOption) (*UpdateMyProfileResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.UpdateMyProfile(ctx, in, opts...)
}

func (m *defaultPassportService) ChangeMyPassword(ctx context.Context, in *ChangeMyPasswordRequest, opts ...grpc.CallOption) (*ChangeMyPasswordResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.ChangeMyPassword(ctx, in, opts...)
}

func (m *defaultPassportService) ResetPasswordByUserID(ctx context.Context, in *ResetPasswordByUserIDRequest, opts ...grpc.CallOption) (*ResetPasswordByUserIDResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.ResetPasswordByUserID(ctx, in, opts...)
}

func (m *defaultPassportService) Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutResponse, error) {
	client := passportpb.NewPassportServiceClient(m.cli.Conn())
	return client.Logout(ctx, in, opts...)
}
