package logic

import (
	"context"

	"hfjj/backend/im-service/internal/wukong"
	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type IssueConnectTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIssueConnectTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IssueConnectTokenLogic {
	return &IssueConnectTokenLogic{ctx: ctx, svcCtx: svcCtx}
}

// IssueConnectToken 负责把平台用户映射成 WuKongIM 用户，并签发连接 token。
func (l *IssueConnectTokenLogic) IssueConnectToken(in *impb.IssueConnectTokenRequest) (*impb.IssueConnectTokenResponse, error) {
	if in.GetUserId() <= 0 {
		return &impb.IssueConnectTokenResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "invalid user id"),
		}, nil
	}

	user, err := l.svcCtx.Repo.GetUserBasic(l.ctx, in.GetUserId())
	if err != nil {
		return nil, err
	}
	if user.AccountStatus == accountStatusBanned || user.AccountStatus == accountStatusClosed {
		return &impb.IssueConnectTokenResponse{
			Base: newStatus(impb.IMCode_IM_CODE_PERMISSION_DENIED, "account disabled"),
		}, nil
	}

	imUID := wukong.FormatIMUID(in.GetUserId())
	binding, err := l.svcCtx.Repo.UpsertBinding(l.ctx, in.GetUserId(), imUID)
	if err != nil {
		return nil, err
	}

	connectToken, route, err := l.svcCtx.WuKongClient.IssueConnectToken(
		l.ctx,
		imUID,
		in.GetDeviceFlag(),
		in.GetDeviceLevel(),
	)
	if err != nil {
		return &impb.IssueConnectTokenResponse{
			Base: newStatus(impb.IMCode_IM_CODE_UPSTREAM_ERROR, err.Error()),
		}, nil
	}

	return &impb.IssueConnectTokenResponse{
		Base: newStatus(impb.IMCode_IM_CODE_OK, "ok"),
		Binding: &impb.IMUserBinding{
			UserId: binding.UserID,
			ImUid:  binding.IMUID,
			Status: binding.Status,
		},
		ConnectToken: connectToken,
		TcpAddr:      route.TCPAddr,
		WsAddr:       route.WSAddr,
		WssAddr:      route.WSSAddr,
	}, nil
}
