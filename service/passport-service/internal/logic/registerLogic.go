package logic

import (
	"context"
	"strings"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
	"hfjj/backend/proto/schoolpb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Register 使用平台注册信息和 school_auth_ticket 完成平台账号注册。
func (l *RegisterLogic) Register(in *passportpb.RegisterRequest) (*passportpb.RegisterResponse, error) {
	rawAccount := in.GetAccount()
	account := normalizeAccount(rawAccount)
	password := in.GetPassword()
	rawNickname := in.GetNickname()
	nickname := normalizeNickname(rawNickname)
	schoolAuthTicket := strings.TrimSpace(in.GetSchoolAuthTicket())

	if account == "" || password == "" || nickname == "" || schoolAuthTicket == "" {
		return &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "account、password、nickname、school_auth_ticket 不能为空"),
		}, nil
	}

	code, message, ok, err := verifyBehaviorToken(l.ctx, l.svcCtx, in.GetBehaviorVerify())
	if err != nil {
		return &passportpb.RegisterResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if !ok {
		return &passportpb.RegisterResponse{
			Base: newStatus(code, message),
		}, nil
	}

	if code, message, ok := validatePlatformAccount(rawAccount); !ok {
		return &passportpb.RegisterResponse{
			Base: newStatus(code, message),
		}, nil
	}

	if code, message, ok := validatePlatformNickname(rawNickname); !ok {
		return &passportpb.RegisterResponse{
			Base: newStatus(code, message),
		}, nil
	}

	if code, message, ok := validatePlatformPassword(password); !ok {
		return &passportpb.RegisterResponse{
			Base: newStatus(code, message),
		}, nil
	}

	user, resp := l.resolveRegistrationUser(account, nickname, password)
	if resp != nil {
		return resp, nil
	}

	bindCtx := metadata.AppendToOutgoingContext(l.ctx, "x-hfjj-register-recovery", "1")
	bindResp, err := l.svcCtx.SchoolRPC.BindPlatformUser(bindCtx, &schoolpb.BindPlatformUserRequest{
		UserId:           user.ID,
		SchoolAuthTicket: schoolAuthTicket,
	})
	if err != nil {
		return &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if bindResp.GetBase().GetCode() != schoolpb.SchoolCode_SCHOOL_CODE_OK {
		return &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_REAUTH_REQUIRED, bindResp.GetBase().GetMessage()),
		}, nil
	}

	user, err = l.finalizeRegisteredUser(user)
	if err != nil {
		return &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	tokens, err := issueNewSessionTokenPair(l.svcCtx, user.ID, "", "")
	if err != nil {
		return &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.RegisterResponse{
		Base:   newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		User:   buildUserProfile(user),
		Tokens: tokens,
	}, nil
}

func (l *RegisterLogic) resolveRegistrationUser(account, nickname, password string) (repository.User, *passportpb.RegisterResponse) {
	existingUser, exists, err := l.svcCtx.UserStore.GetByAccount(account)
	if err != nil {
		return repository.User{}, &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}
	}
	if exists {
		if existingUser.SchoolBound {
			return repository.User{}, &passportpb.RegisterResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_ACCOUNT_ALREADY_EXISTS, "账号已存在"),
			}
		}
		if !repository.VerifyPassword(password, existingUser.PasswordHash) {
			return repository.User{}, &passportpb.RegisterResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_ACCOUNT_ALREADY_EXISTS, "账号已存在"),
			}
		}
		return existingUser, nil
	}

	existsNickname, err := l.svcCtx.UserStore.ExistsNickname(nickname)
	if err != nil {
		return repository.User{}, &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}
	}
	if existsNickname {
		return repository.User{}, &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_NICKNAME_ALREADY_EXISTS, "昵称已存在"),
		}
	}

	passwordHash, err := repository.HashPassword(password)
	if err != nil {
		return repository.User{}, &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}
	}

	user, err := l.svcCtx.UserStore.Create(repository.User{
		Account:       account,
		Nickname:      nickname,
		PasswordHash:  passwordHash,
		AccountStatus: int32(passportpb.AccountStatus_ACCOUNT_STATUS_NORMAL),
		SchoolBound:   false,
	})
	if err != nil {
		return repository.User{}, &passportpb.RegisterResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}
	}
	return user, nil
}

func (l *RegisterLogic) finalizeRegisteredUser(user repository.User) (repository.User, error) {
	if !user.SchoolBound {
		user.SchoolBound = true
		if err := l.svcCtx.UserStore.Save(user); err != nil {
			return repository.User{}, err
		}
	}

	if _, err := l.svcCtx.Enforcer.AddRoleForUser(casbinSubject(user.ID), "user"); err != nil {
		return repository.User{}, err
	}
	if err := l.svcCtx.AuthzStore.Delete(user.ID); err != nil {
		return repository.User{}, err
	}
	if err := l.svcCtx.PublishCasbinPolicyReload(); err != nil {
		return repository.User{}, err
	}

	return user, nil
}
