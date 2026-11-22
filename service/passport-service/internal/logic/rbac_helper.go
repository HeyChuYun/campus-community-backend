package logic

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

type permissionMeta struct {
	Name        string
	Module      string
	Description string
}

var roleNameMapping = map[string]string{
	"user":       "普通用户",
	"admin":      "管理员",
	"Premium":    "青衿",
	"PremiumPro": "鸿儒",
}

var permissionMetaMapping = map[string]permissionMeta{
	"relation.read":                 {Name: "查看关系", Module: "relation", Description: "允许查看关系快照"},
	"relation.following.read":       {Name: "查看关注列表", Module: "relation", Description: "允许查看关注列表"},
	"relation.followers.read":       {Name: "查看粉丝列表", Module: "relation", Description: "允许查看粉丝列表"},
	"relation.follow.create":        {Name: "关注用户", Module: "relation", Description: "允许关注目标用户"},
	"relation.follow.delete":        {Name: "取消关注", Module: "relation", Description: "允许取消关注目标用户"},
	"relation.block.create":         {Name: "拉黑用户", Module: "relation", Description: "允许拉黑目标用户"},
	"relation.block.delete":         {Name: "取消拉黑", Module: "relation", Description: "允许取消拉黑目标用户"},
	"content.post.read":             {Name: "查看帖子详情", Module: "content", Description: "允许查看帖子详情"},
	"content.feed.read":             {Name: "查看内容流", Module: "content", Description: "允许查看关注流和发现流"},
	"content.post.create":           {Name: "创建帖子", Module: "content", Description: "允许发帖"},
	"content.post.delete.own":       {Name: "删除自己的帖子", Module: "content", Description: "允许删除自己发布的帖子"},
	"content.post.delete.any":       {Name: "删除任意帖子", Module: "content", Description: "允许删除任意帖子"},
	"content.comment.create":        {Name: "创建评论", Module: "content", Description: "允许发表评论"},
	"content.comment.delete.own":    {Name: "删除自己的评论", Module: "content", Description: "允许删除自己的评论"},
	"content.comment.delete.any":    {Name: "删除任意评论", Module: "content", Description: "允许删除任意评论"},
	"content.like.create":           {Name: "点赞", Module: "content", Description: "允许点赞"},
	"content.favorite.create":       {Name: "收藏", Module: "content", Description: "允许收藏"},
	"message.send":                  {Name: "发送私信", Module: "message", Description: "允许发送私信"},
	"profile.update":                {Name: "更新个人资料", Module: "profile", Description: "允许修改个人资料"},
	"school.profile.read":           {Name: "查看学校个人信息", Module: "school", Description: "允许查看学校个人信息"},
	"school.schedule.read":          {Name: "查看课表", Module: "school", Description: "允许查看课表"},
	"school.grade.read":             {Name: "查看成绩", Module: "school", Description: "允许查看成绩"},
	"admin.user.read":               {Name: "查看用户", Module: "admin", Description: "允许查看用户"},
	"admin.user.role.assign":        {Name: "分配用户角色", Module: "admin", Description: "允许给用户分配角色"},
	"admin.user.ban":                {Name: "封禁用户", Module: "admin", Description: "允许封禁用户"},
	"admin.post.review.read":        {Name: "查看帖子内容管理", Module: "admin", Description: "允许查看帖子内容管理列表与违规记录"},
	"admin.post.review.manage":      {Name: "处理帖子违规", Module: "admin", Description: "允许对帖子执行违规封禁与禁言处置"},
	"admin.content.moderate":        {Name: "审核内容", Module: "admin", Description: "允许审核内容"},
	"admin.ad.review":               {Name: "审核广告", Module: "admin", Description: "允许审核帖子广告订单"},
	"admin.membership.read":         {Name: "查看会员", Module: "admin", Description: "允许查看会员详情与订单"},
	"admin.membership.manage":       {Name: "管理会员", Module: "admin", Description: "允许后台发放、续期、撤销会员"},
	"membership.premium.active":     {Name: "青衿有效", Module: "membership", Description: "青衿增值服务有效"},
	"membership.premium_pro.active": {Name: "鸿儒有效", Module: "membership", Description: "鸿儒增值服务有效"},
	"monetization.ads.reduced":      {Name: "轻度减广告", Module: "monetization", Description: "减少信息流广告展示"},
	"monetization.ads.free":         {Name: "免信息流广告", Module: "monetization", Description: "免除信息流广告展示"},
}

// loadUserByAccessToken 统一校验 access_token，并返回当前平台用户。
func loadUserByAccessToken(svcCtx *svc.ServiceContext, accessToken string) (repository.User, passportpb.PassportCode, string, bool, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "access_token 不能为空", false, nil
	}

	claims, err := parseAccessToken(svcCtx, accessToken)
	if err != nil {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID, "access_token 无效或已过期", false, nil
	}

	session, ok, err := svcCtx.SessionStore.Get(claims.SessionID)
	if err != nil {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error(), false, err
	}
	if !ok ||
		time.Now().After(session.ExpiresAt) ||
		!session.RevokedAt.IsZero() ||
		session.UserID != claims.UserID ||
		session.AccessJTI != claims.ID {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID, "access_token 对应会话无效", false, nil
	}

	user, ok, err := svcCtx.UserStore.GetByID(claims.UserID)
	if err != nil {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error(), false, err
	}
	if !ok {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID, "access_token 无效", false, nil
	}
	user, _, err = syncAccountRestrictionState(context.Background(), svcCtx, user)
	if err != nil {
		return repository.User{}, passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error(), false, err
	}
	return user, passportpb.PassportCode_PASSPORT_CODE_OK, "ok", true, nil
}

// loadAuthorizationSnapshot 优先读取缓存中的角色和权限快照。
func loadAuthorizationSnapshot(svcCtx *svc.ServiceContext, user repository.User) (*passportpb.AuthorizationSnapshot, error) {
	if cached, ok, err := svcCtx.AuthzStore.Get(user.ID); err == nil && ok {
		return &passportpb.AuthorizationSnapshot{
			User:        buildUserProfile(user),
			Roles:       buildRoleInfos(cached.Roles),
			Permissions: buildPermissionInfos(cached.PermissionCodes),
		}, nil
	} else if err != nil {
		return nil, err
	}
	return rebuildAuthorizationSnapshot(svcCtx, user)
}

// rebuildAuthorizationSnapshot 强制回源 Casbin，重建并刷新当前用户的授权快照。
func rebuildAuthorizationSnapshot(svcCtx *svc.ServiceContext, user repository.User) (*passportpb.AuthorizationSnapshot, error) {
	roles, err := svcCtx.Enforcer.GetRolesForUser(casbinSubject(user.ID))
	if err != nil {
		return nil, err
	}
	policies, err := svcCtx.Enforcer.GetImplicitPermissionsForUser(casbinSubject(user.ID))
	if err != nil {
		return nil, err
	}

	permissionCodes := make([]string, 0, len(policies))
	seen := make(map[string]struct{}, len(policies))
	for _, policy := range policies {
		if len(policy) < 3 {
			continue
		}
		permissionCode := policy[1] + "." + policy[2]
		if _, ok := seen[permissionCode]; ok {
			continue
		}
		seen[permissionCode] = struct{}{}
		permissionCodes = append(permissionCodes, permissionCode)
	}
	slices.Sort(roles)
	slices.Sort(permissionCodes)

	if err := svcCtx.AuthzStore.Put(user.ID, repository.AuthorizationSnapshot{
		Roles:           roles,
		PermissionCodes: permissionCodes,
	}, 5*time.Minute); err != nil {
		return nil, err
	}

	return &passportpb.AuthorizationSnapshot{
		User:        buildUserProfile(user),
		Roles:       buildRoleInfos(roles),
		Permissions: buildPermissionInfos(permissionCodes),
	}, nil
}

func snapshotContainsPermission(snapshot *passportpb.AuthorizationSnapshot, permissionCode string) bool {
	permissionCode = strings.TrimSpace(permissionCode)
	if snapshot == nil || permissionCode == "" {
		return false
	}
	for _, item := range snapshot.GetPermissions() {
		if strings.TrimSpace(item.GetPermissionCode()) == permissionCode {
			return true
		}
	}
	return false
}

// decideAuthorization 根据账号状态、Casbin 策略和资源归属关系决定是否允许访问。
func decideAuthorization(svcCtx *svc.ServiceContext, user repository.User, permissionCode string, resourceOwnerUserID int64) (bool, passportpb.PassportCode, string) {
	permissionCode = strings.TrimSpace(permissionCode)
	if permissionCode == "" {
		return false, passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "permission_code 不能为空"
	}

	switch user.AccountStatus {
	case int32(passportpb.AccountStatus_ACCOUNT_STATUS_BLOCKED), int32(passportpb.AccountStatus_ACCOUNT_STATUS_DEACTIVATED):
		return false, passportpb.PassportCode_PASSPORT_CODE_ACCOUNT_UNAVAILABLE, "账号不可用"
	case int32(passportpb.AccountStatus_ACCOUNT_STATUS_READ_ONLY):
		if isWritePermission(permissionCode) {
			return false, passportpb.PassportCode_PASSPORT_CODE_PERMISSION_DENIED, "当前账号为只读状态"
		}
	}

	object, action, err := splitPermissionCode(permissionCode)
	if err != nil {
		return false, passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, err.Error()
	}

	if strings.HasSuffix(permissionCode, ".own") {
		if resourceOwnerUserID <= 0 {
			return false, passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "own 权限缺少资源归属 user_id"
		}
		if resourceOwnerUserID != user.ID {
			anyPermissionCode := strings.TrimSuffix(permissionCode, ".own") + ".any"
			anyObject, anyAction, err := splitPermissionCode(anyPermissionCode)
			if err != nil {
				return false, passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()
			}
			allowedAny, err := svcCtx.Enforcer.Enforce(casbinSubject(user.ID), anyObject, anyAction)
			if err != nil {
				return false, passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()
			}
			if !allowedAny {
				return false, passportpb.PassportCode_PASSPORT_CODE_PERMISSION_DENIED, "只能操作自己的资源"
			}
			return true, passportpb.PassportCode_PASSPORT_CODE_OK, "ok"
		}
	}

	allowed, err := svcCtx.Enforcer.Enforce(casbinSubject(user.ID), object, action)
	if err != nil {
		return false, passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()
	}
	if !allowed {
		return false, passportpb.PassportCode_PASSPORT_CODE_PERMISSION_DENIED, "权限不足"
	}
	return true, passportpb.PassportCode_PASSPORT_CODE_OK, "ok"
}

// isWritePermission 判断权限点是否属于写操作。
func isWritePermission(permissionCode string) bool {
	if strings.HasSuffix(permissionCode, ".read") {
		return false
	}
	writeKeywords := []string{
		".create",
		".update",
		".delete",
		".assign",
		".moderate",
		".ban",
		".send",
	}
	for _, keyword := range writeKeywords {
		if strings.Contains(permissionCode, keyword) {
			return true
		}
	}
	return true
}

func casbinSubject(userID int64) string {
	return strconv.FormatInt(userID, 10)
}

func buildRoleInfos(roleCodes []string) []*passportpb.RoleInfo {
	items := make([]*passportpb.RoleInfo, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		items = append(items, &passportpb.RoleInfo{
			RoleCode: roleCode,
			RoleName: roleName(roleCode),
		})
	}
	return items
}

func buildPermissionInfos(permissionCodes []string) []*passportpb.PermissionInfo {
	items := make([]*passportpb.PermissionInfo, 0, len(permissionCodes))
	for _, permissionCode := range permissionCodes {
		meta := permissionMetadata(permissionCode)
		items = append(items, &passportpb.PermissionInfo{
			PermissionCode: permissionCode,
			PermissionName: meta.Name,
			Module:         meta.Module,
			Description:    meta.Description,
		})
	}
	return items
}

func roleName(roleCode string) string {
	if name, ok := roleNameMapping[roleCode]; ok {
		return name
	}
	return roleCode
}

func permissionMetadata(permissionCode string) permissionMeta {
	if meta, ok := permissionMetaMapping[permissionCode]; ok {
		return meta
	}
	return permissionMeta{
		Name:        permissionCode,
		Module:      "",
		Description: "",
	}
}
