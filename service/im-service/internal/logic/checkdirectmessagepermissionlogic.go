package logic

import (
	"context"

	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type CheckDirectMessagePermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCheckDirectMessagePermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckDirectMessagePermissionLogic {
	return &CheckDirectMessagePermissionLogic{ctx: ctx, svcCtx: svcCtx}
}

// CheckDirectMessagePermission 负责在真正发消息前检查私信规则。
func (l *CheckDirectMessagePermissionLogic) CheckDirectMessagePermission(in *impb.CheckDirectMessagePermissionRequest) (*impb.CheckDirectMessagePermissionResponse, error) {
	permission, code, message, err := resolveDirectMessagePermission(l.ctx, l.svcCtx, in.GetSenderUserId(), in.GetReceiverUserId())
	if err != nil {
		return nil, err
	}
	return &impb.CheckDirectMessagePermissionResponse{
		Base:       newStatus(code, message),
		Permission: permission,
	}, nil
}
