package logic

import (
	"context"
	"strings"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

// verifyBehaviorToken 在登录和注册前统一校验极验行为验证结果。
// gateway 只负责透传字段，真正的人机校验细节由 passport-service 负责。
func verifyBehaviorToken(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	token *passportpb.BehaviorVerifyToken,
) (passportpb.PassportCode, string, bool, error) {
	if svcCtx == nil || svcCtx.BehaviorVerifier == nil {
		return passportpb.PassportCode_PASSPORT_CODE_OK, "ok", true, nil
	}

	payload := repository.BehaviorVerifyPayload{}
	if token != nil {
		payload = repository.BehaviorVerifyPayload{
			LotNumber:     token.GetLotNumber(),
			CaptchaOutput: token.GetCaptchaOutput(),
			PassToken:     token.GetPassToken(),
			GenTime:       token.GetGenTime(),
		}
	}

	result, err := svcCtx.BehaviorVerifier.Verify(ctx, payload)
	if err != nil {
		return passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, "行为验证服务异常，请稍后重试", false, err
	}
	if !result.Passed {
		message := strings.TrimSpace(result.Reason)
		if message == "" {
			message = "行为验证未通过"
		}
		return passportpb.PassportCode_PASSPORT_CODE_BEHAVIOR_VERIFY_FAILED, message, false, nil
	}

	return passportpb.PassportCode_PASSPORT_CODE_OK, "ok", true, nil
}
