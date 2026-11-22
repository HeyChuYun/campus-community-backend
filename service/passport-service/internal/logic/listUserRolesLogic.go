package logic

import (
	"context"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUserRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUserRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserRolesLogic {
	return &ListUserRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListUserRoles 通过 Casbin 查询指定用户当前拥有的角色。
func (l *ListUserRolesLogic) ListUserRoles(in *passportpb.ListUserRolesRequest) (*passportpb.ListUserRolesResponse, error) {
	if in.GetUserId() <= 0 {
		return &passportpb.ListUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "user_id 必须大于 0"),
		}, nil
	}

	roleCodes, err := l.svcCtx.Enforcer.GetRolesForUser(casbinSubject(in.GetUserId()))
	if err != nil {
		return &passportpb.ListUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.ListUserRolesResponse{
		Base:  newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Roles: buildRoleInfos(roleCodes),
	}, nil
}
