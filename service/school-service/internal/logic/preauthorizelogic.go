package logic

import (
	"context"

	"hfjj/backend/proto/schoolpb"
	"hfjj/backend/school-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreAuthorizeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPreAuthorizeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreAuthorizeLogic {
	return &PreAuthorizeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PreAuthorize 先向学校侧请求验证码，并把学校会话缓存在内存里。
func (l *PreAuthorizeLogic) PreAuthorize(in *schoolpb.PreAuthorizeRequest) (*schoolpb.PreAuthorizeResponse, error) {
	// 第一步：访问学校登录页，建立学校侧会话并获取验证码图片。
	session, err := l.svcCtx.CampusClient.PreAuthorize(l.ctx)
	if err != nil {
		return &schoolpb.PreAuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}

	// 第二步：把 auth_session_id 对应的 cookies 和验证码 uid 暂存在内存里，供后续 Authorize 使用。
	if err := l.svcCtx.AuthStore.Set(l.ctx, session.AuthSessionID, session); err != nil {
		return &schoolpb.PreAuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}

	// 第三步：把前端真正需要展示的验证码图片和 auth_session_id 返回出去。
	return &schoolpb.PreAuthorizeResponse{
		Base:               newStatus(schoolpb.SchoolCode_SCHOOL_CODE_OK, "ok"),
		AuthSessionId:      session.AuthSessionID,
		CaptchaImageBase64: session.CaptchaBase64,
	}, nil
}
