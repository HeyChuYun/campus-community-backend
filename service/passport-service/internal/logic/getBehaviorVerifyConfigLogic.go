package logic

import (
	"context"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBehaviorVerifyConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBehaviorVerifyConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBehaviorVerifyConfigLogic {
	return &GetBehaviorVerifyConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBehaviorVerifyConfig 返回登录和注册页初始化行为验证所需的公开配置。
func (l *GetBehaviorVerifyConfigLogic) GetBehaviorVerifyConfig(
	_ *passportpb.GetBehaviorVerifyConfigRequest,
) (*passportpb.GetBehaviorVerifyConfigResponse, error) {
	config := passportpb.BehaviorVerifyConfig{
		Enabled: false,
	}
	if l.svcCtx.BehaviorVerifier != nil {
		publicConfig := l.svcCtx.BehaviorVerifier.PublicConfig()
		config.Enabled = publicConfig.Enabled
		config.Provider = publicConfig.Provider
		config.CaptchaId = publicConfig.CaptchaID
	}

	return &passportpb.GetBehaviorVerifyConfigResponse{
		Base:   newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Config: &config,
	}, nil
}
