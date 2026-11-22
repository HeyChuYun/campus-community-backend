package logic

import (
	"context"
	"strings"
	"time"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RefreshToken 只负责平台 refresh_token 续期，不参与学校侧 session 校验。
func (l *RefreshTokenLogic) RefreshToken(in *passportpb.RefreshTokenRequest) (*passportpb.RefreshTokenResponse, error) {
	// 第一步：校验 refresh_token 入参。
	if strings.TrimSpace(in.GetRefreshToken()) == "" {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "refresh_token 不能为空"),
		}, nil
	}

	// 第二步：读取 refresh_token 记录，并判断是否存在或已过期。
	record, ok, err := l.svcCtx.TokenStore.GetRefresh(in.GetRefreshToken())
	if err != nil {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !ok || time.Now().After(record.ExpiresAt) {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_REFRESH_TOKEN_INVALID, "refresh_token 无效或已过期"),
		}, nil
	}

	// 第三步：根据 refresh_token 中记录的 user_id 读取平台用户。
	user, ok, err := l.svcCtx.UserStore.GetByID(record.UserID)
	if err != nil {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !ok {
		_ = l.svcCtx.TokenStore.DeleteRefresh(in.GetRefreshToken())
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_REFRESH_TOKEN_INVALID, "refresh_token 无效"),
		}, nil
	}

	// 第四步：读取当前 refresh_token 绑定的 session，确认该登录会话仍然有效。
	session, ok, err := l.svcCtx.SessionStore.Get(record.SessionID)
	if err != nil {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !ok || time.Now().After(session.ExpiresAt) || !session.RevokedAt.IsZero() || session.UserID != user.ID || session.RefreshJTI != record.JTI {
		_ = l.svcCtx.TokenStore.DeleteRefresh(in.GetRefreshToken())
		if ok {
			_ = l.svcCtx.SessionStore.Delete(record.SessionID)
		}
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_REFRESH_TOKEN_INVALID, "refresh_token 对应会话无效"),
		}, nil
	}

	// 第五步：旧 refresh_token 在轮换时立即撤销，防止被重复使用。
	if err := l.svcCtx.TokenStore.DeleteRefresh(in.GetRefreshToken()); err != nil {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	// 第六步：沿用原 session_id 轮换新的 access_jti / refresh_jti，实现整条会话链路可撤销。
	tokens, err := rotateSessionTokenPair(l.svcCtx, session)
	if err != nil {
		return &passportpb.RefreshTokenResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.RefreshTokenResponse{
		Base:   newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Tokens: tokens,
	}, nil
}
