package logic

import (
	"context"
	"strings"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Logout 主动撤销 refresh_token。
func (l *LogoutLogic) Logout(in *passportpb.LogoutRequest) (*passportpb.LogoutResponse, error) {
	// 第一步：校验 refresh_token 入参。
	if strings.TrimSpace(in.GetRefreshToken()) == "" {
		return &passportpb.LogoutResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "refresh_token 不能为空"),
		}, nil
	}

	// 第二步：先读取 refresh_token 绑定的 session，便于后续把整条会话链路一起注销。
	record, ok, err := l.svcCtx.TokenStore.GetRefresh(in.GetRefreshToken())
	if err != nil {
		return &passportpb.LogoutResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if ok {
		// 删除 refresh_token 后，即使客户端继续拿旧 refresh_token 刷新，也会立即失败。
		if err := l.svcCtx.TokenStore.DeleteRefresh(in.GetRefreshToken()); err != nil {
			return &passportpb.LogoutResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			}, nil
		}

		// 再删除 session 状态，使当前 JWT access_token 因查不到 session 而立刻失效。
		if err := l.svcCtx.SessionStore.Delete(record.SessionID); err != nil {
			return &passportpb.LogoutResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			}, nil
		}
	}

	// 第三步：返回退出成功。
	return &passportpb.LogoutResponse{
		Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
	}, nil
}
