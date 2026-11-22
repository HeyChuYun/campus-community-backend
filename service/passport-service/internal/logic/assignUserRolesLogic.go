package logic

import (
	"context"
	"strings"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignUserRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignUserRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignUserRolesLogic {
	return &AssignUserRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AssignUserRoles 使用 Casbin 的 g 规则全量替换目标用户角色。
// 操作者身份完全由 operator_access_token 决定，不再信任外部直接传 operator_user_id。
func (l *AssignUserRolesLogic) AssignUserRoles(in *passportpb.AssignUserRolesRequest) (*passportpb.AssignUserRolesResponse, error) {
	// 第一步：校验基础参数。
	if strings.TrimSpace(in.GetOperatorAccessToken()) == "" || in.GetTargetUserId() <= 0 {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "operator_access_token 不能为空，target_user_id 必须大于 0"),
		}, nil
	}

	roleCodes := trimRoleCodes(in.GetRoleCodes())
	if len(roleCodes) == 0 {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "role_codes 不能为空"),
		}, nil
	}
	for _, roleCode := range roleCodes {
		if _, ok := roleNameMapping[roleCode]; !ok {
			return &passportpb.AssignUserRolesResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "存在未知角色: "+roleCode),
			}, nil
		}
	}

	// 第二步：通过 access_token 解析操作者身份，并校验其是否具备角色分配权限。
	operator, code, message, ok, err := loadUserByAccessToken(l.svcCtx, in.GetOperatorAccessToken())
	if err != nil {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if !ok {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(code, message),
		}, nil
	}

	allowed, authCode, denyReason := decideAuthorization(l.svcCtx, operator, "admin.user.role.assign", 0)
	if !allowed {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(authCode, denyReason),
		}, nil
	}

	// 第三步：确认目标用户存在。
	if _, ok, err := l.svcCtx.UserStore.GetByID(in.GetTargetUserId()); err != nil {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	} else if !ok {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "target_user_id 不存在"),
		}, nil
	}

	// 第四步：删除目标用户现有角色，再写入新的角色集合。
	targetSubject := casbinSubject(in.GetTargetUserId())
	currentRoles, err := l.svcCtx.Enforcer.GetRolesForUser(targetSubject)
	if err != nil {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	for _, roleCode := range currentRoles {
		if _, err := l.svcCtx.Enforcer.DeleteRoleForUser(targetSubject, roleCode); err != nil {
			return &passportpb.AssignUserRolesResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			}, nil
		}
	}
	for _, roleCode := range roleCodes {
		if _, err := l.svcCtx.Enforcer.AddRoleForUser(targetSubject, roleCode); err != nil {
			return &passportpb.AssignUserRolesResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			}, nil
		}
	}

	// 第五步：写入角色变更审计日志，便于排查谁把谁设成了 admin。
	if err := l.svcCtx.RoleChangeLogStore.Create(repository.UserRoleChangeLog{
		OperatorUserID: operator.ID,
		TargetUserID:   in.GetTargetUserId(),
		BeforeRoles:    currentRoles,
		AfterRoles:     roleCodes,
	}); err != nil {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if err := l.svcCtx.AuthzStore.Delete(in.GetTargetUserId()); err != nil {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	// 第六步：广播 Casbin 策略刷新，通知其他 passport-service 实例重新加载策略。
	if err := l.svcCtx.PublishCasbinPolicyReload(); err != nil {
		return &passportpb.AssignUserRolesResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.AssignUserRolesResponse{
		Base:  newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		Roles: buildRoleInfos(roleCodes),
	}, nil
}

func trimRoleCodes(roleCodes []string) []string {
	seen := make(map[string]struct{}, len(roleCodes))
	items := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleCode = strings.TrimSpace(roleCode)
		if roleCode == "" {
			continue
		}
		if _, ok := seen[roleCode]; ok {
			continue
		}
		seen[roleCode] = struct{}{}
		items = append(items, roleCode)
	}
	return items
}
