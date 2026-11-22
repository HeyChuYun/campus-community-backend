package logic

import (
	"context"
	"strings"
	"time"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Login 使用平台账号或学号登录，并签发双 token。
func (l *LoginLogic) Login(in *passportpb.LoginRequest) (*passportpb.LoginResponse, error) {
	rawAccount := in.GetAccount()
	account := normalizeAccount(rawAccount)
	password := in.GetPassword()
	if account == "" || password == "" {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号或学号、密码不能为空"),
		}, nil
	}
	if code, message, ok := validateLoginAccount(rawAccount); !ok {
		return &passportpb.LoginResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if containsWhitespace(password) {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "密码不能包含空格"),
		}, nil
	}

	code, message, ok, err := verifyBehaviorToken(l.ctx, l.svcCtx, in.GetBehaviorVerify())
	if err != nil {
		return &passportpb.LoginResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if !ok {
		return &passportpb.LoginResponse{
			Base: newStatus(code, message),
		}, nil
	}

	user, ok, err := l.findLoginUser(account)
	if err != nil {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !ok || !repository.VerifyPassword(password, user.PasswordHash) {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_CREDENTIAL, "账号或密码错误"),
		}, nil
	}

	if repository.PasswordNeedsRehash(user.PasswordHash) {
		passwordHash, err := repository.HashPassword(password)
		if err != nil {
			return &passportpb.LoginResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			}, nil
		}
		user.PasswordHash = passwordHash
	}

	activeBanEndsAt := (*time.Time)(nil)
	user, activeBanEndsAt, err = syncAccountRestrictionState(l.ctx, l.svcCtx, user)
	if err != nil {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	if user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_BLOCKED) ||
		user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_DEACTIVATED) {
		message := "账号不可用"
		if activeBanEndsAt != nil {
			message = "账号已被封禁，请于 " + activeBanEndsAt.Local().Format("2006-01-02 15:04:05") + " 后重试"
		}
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_ACCOUNT_UNAVAILABLE, message),
		}, nil
	}
	if !user.SchoolBound {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_REAUTH_REQUIRED, "账号注册尚未完成，请重新完成校园授权"),
		}, nil
	}

	user.LastLoginAt = time.Now()
	if err := l.svcCtx.UserStore.Save(user); err != nil {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	loginIP, device := loginMetadata(l.ctx)
	if err := l.svcCtx.LoginStore.Create(repository.LoginRecord{
		UserID:  user.ID,
		LoginIP: loginIP,
		Device:  device,
		LoginAt: user.LastLoginAt,
	}); err != nil {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	tokens, err := issueNewSessionTokenPair(l.svcCtx, user.ID, device, loginIP)
	if err != nil {
		return &passportpb.LoginResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.LoginResponse{
		Base:   newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		User:   buildUserProfile(user),
		Tokens: tokens,
	}, nil
}

func (l *LoginLogic) findLoginUser(account string) (repository.User, bool, error) {
	if isAllDigits(account) {
		return l.svcCtx.UserStore.GetByStudentNo(account)
	}
	return l.svcCtx.UserStore.GetByAccount(account)
}

func loginMetadata(ctx context.Context) (string, string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ""
	}
	return firstMetadata(md, "x-login-ip"), firstMetadata(md, "x-device")
}

func firstMetadata(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
