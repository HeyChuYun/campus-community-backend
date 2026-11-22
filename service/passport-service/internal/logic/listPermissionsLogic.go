package logic

import (
	"context"
	"slices"
	"strings"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPermissionsLogic {
	return &ListPermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPermissions 返回系统内置权限点定义，可按模块过滤。
func (l *ListPermissionsLogic) ListPermissions(in *passportpb.ListPermissionsRequest) (*passportpb.ListPermissionsResponse, error) {
	module := strings.TrimSpace(in.GetModule())
	permissionCodes := make([]string, 0, len(permissionMetaMapping))
	for permissionCode, meta := range permissionMetaMapping {
		if module != "" && meta.Module != module {
			continue
		}
		permissionCodes = append(permissionCodes, permissionCode)
	}
	slices.Sort(permissionCodes)

	return &passportpb.ListPermissionsResponse{
		Base:        newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Permissions: buildPermissionInfos(permissionCodes),
	}, nil
}
