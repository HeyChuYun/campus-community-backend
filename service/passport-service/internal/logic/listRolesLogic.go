package logic

import (
	"context"
	"slices"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRolesLogic {
	return &ListRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRoles 返回系统内置角色定义。
func (l *ListRolesLogic) ListRoles(_ *passportpb.ListRolesRequest) (*passportpb.ListRolesResponse, error) {
	roleCodes := make([]string, 0, len(roleNameMapping))
	for roleCode := range roleNameMapping {
		roleCodes = append(roleCodes, roleCode)
	}
	slices.Sort(roleCodes)

	return &passportpb.ListRolesResponse{
		Base:  newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Roles: buildRoleInfos(roleCodes),
	}, nil
}
