package logic

import (
	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

func updatePlatformPassword(
	svcCtx *svc.ServiceContext,
	user repository.User,
	newPassword string,
) (passportpb.PassportCode, string, error) {
	if code, message, ok := validatePlatformPassword(newPassword); !ok {
		return code, message, nil
	}

	if repository.VerifyPassword(newPassword, user.PasswordHash) {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "新密码不能与当前密码相同", nil
	}

	passwordHash, err := repository.HashPassword(newPassword)
	if err != nil {
		return passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error(), err
	}

	user.PasswordHash = passwordHash
	if err := svcCtx.UserStore.Save(user); err != nil {
		return passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error(), err
	}
	return passportpb.PassportCode_PASSPORT_CODE_OK, "ok", nil
}
