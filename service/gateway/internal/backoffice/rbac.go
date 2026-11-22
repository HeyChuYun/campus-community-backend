package backoffice

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

var (
	ErrInvalidRole       = errors.New("invalid role")
	ErrSystemRole        = errors.New("system role cannot be deleted")
	ErrRoleExists        = errors.New("role already exists")
	ErrRoleNotFound      = errors.New("role not found")
	ErrPermissionUnknown = errors.New("unknown permission")
	ErrUserNotFound      = errors.New("user not found")
)

type PermissionMeta struct {
	Name        string
	Module      string
	Description string
}

type RoleItem struct {
	RoleCode        string   `json:"role_code"`
	RoleName        string   `json:"role_name"`
	IsSystem        bool     `json:"is_system"`
	UserCount       int64    `json:"user_count"`
	PermissionCount int      `json:"permission_count"`
	Permissions     []string `json:"permissions"`
	CreatedAt       string   `json:"created_at,omitempty"`
}

type PermissionItem struct {
	PermissionCode string `json:"permission_code"`
	PermissionName string `json:"permission_name"`
	Module         string `json:"module"`
	Description    string `json:"description"`
}

type AssignUserRolesResult struct {
	UserID    int64
	RoleCodes []string
	RoleNames []string
}

type RoleCatalog struct {
	defaultRoleNames map[string]string
	systemRoles      map[string]bool
	permissions      map[string]PermissionMeta
}

type RBACService struct {
	db          *gorm.DB
	redis       *redis.Client
	redisPrefix string
	catalog     RoleCatalog
}

type adminRoleRecord struct {
	RoleCode  string `gorm:"column:role_code;primaryKey;type:varchar(100)"`
	RoleName  string `gorm:"column:role_name;type:varchar(100);not null;default:''"`
	IsSystem  bool   `gorm:"column:is_system;not null;default:false;index:idx_admin_roles_is_system"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (adminRoleRecord) TableName() string {
	return "admin_roles"
}

type casbinRuleRecord struct {
	ID    uint   `gorm:"column:id;primaryKey"`
	PType string `gorm:"column:ptype"`
	V0    string `gorm:"column:v0"`
	V1    string `gorm:"column:v1"`
	V2    string `gorm:"column:v2"`
}

func (casbinRuleRecord) TableName() string {
	return "casbin_rule"
}

func NewRBACService(db *gorm.DB, redisClient *redis.Client, redisPrefix string) *RBACService {
	return &RBACService{
		db:          requireGormDB(db),
		redis:       requireRedisClient(redisClient),
		redisPrefix: redisPrefix,
		catalog:     defaultRoleCatalog(),
	}
}

func (s *RBACService) ListRoles(ctx context.Context) ([]RoleItem, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return s.listRoles(ctx)
}

func (s *RBACService) CreateRole(ctx context.Context, roleCode, roleName string) ([]RoleItem, error) {
	roleCode = normalizeRoleCode(roleCode)
	if !isValidRoleCode(roleCode) {
		return nil, ErrInvalidRole
	}
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		roleName = roleCode
	}
	if len([]rune(roleName)) > 100 {
		return nil, ErrInvalidRole
	}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	exists, err := s.roleExists(ctx, roleCode)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRoleExists
	}
	if err := s.db.WithContext(ctx).Table("admin_roles").Create(map[string]any{
		"role_code":  roleCode,
		"role_name":  roleName,
		"is_system":  false,
		"created_at": gorm.Expr("NOW()"),
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return nil, err
	}
	return s.listRoles(ctx)
}

func (s *RBACService) DeleteRole(ctx context.Context, roleCode string) error {
	roleCode = normalizeRoleCode(roleCode)
	if s.catalog.systemRoles[roleCode] {
		return ErrSystemRole
	}
	if !isValidRoleCode(roleCode) {
		return ErrInvalidRole
	}
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}
	exists, err := s.roleExists(ctx, roleCode)
	if err != nil {
		return err
	}
	if !exists {
		return ErrRoleNotFound
	}
	affectedUserIDs, err := s.userIDsForRole(ctx, roleCode)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_code = ?", roleCode).Delete(&adminRoleRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("(ptype = ? AND v0 = ?) OR (ptype = ? AND v1 = ?)", "p", roleCode, "g", roleCode).
			Delete(&casbinRuleRecord{}).Error
	}); err != nil {
		return err
	}
	s.publishChanged(ctx, affectedUserIDs)
	return nil
}

func (s *RBACService) ListPermissions(ctx context.Context) ([]PermissionItem, error) {
	return s.listPermissions(ctx)
}

func (s *RBACService) SetRolePermissions(ctx context.Context, roleCode string, requested []string) ([]RoleItem, error) {
	roleCode = normalizeRoleCode(roleCode)
	if !isValidRoleCode(roleCode) {
		return nil, ErrInvalidRole
	}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	exists, err := s.roleExists(ctx, roleCode)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRoleNotFound
	}
	known, err := s.knownPermissionCodes(ctx)
	if err != nil {
		return nil, err
	}
	permissionCodes, err := normalizePermissionCodes(requested, known)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("ptype = ? AND v0 = ?", "p", roleCode).Delete(&casbinRuleRecord{}).Error; err != nil {
			return err
		}
		for _, permissionCode := range permissionCodes {
			object, action, splitErr := splitPermissionCode(permissionCode)
			if splitErr != nil {
				return splitErr
			}
			if err := tx.Create(&casbinRuleRecord{
				PType: "p",
				V0:    roleCode,
				V1:    object,
				V2:    action,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if affectedUserIDs, err := s.userIDsForRole(ctx, roleCode); err == nil {
		s.publishChanged(ctx, affectedUserIDs)
	}
	return s.listRoles(ctx)
}

func (s *RBACService) AssignUserRoles(ctx context.Context, targetUserID int64, requested []string) (AssignUserRolesResult, error) {
	if targetUserID <= 0 {
		return AssignUserRolesResult{}, ErrInvalidInput
	}
	if err := s.ensureSchema(ctx); err != nil {
		return AssignUserRolesResult{}, err
	}
	roleCodes, err := normalizeRoleCodes(requested)
	if err != nil {
		return AssignUserRolesResult{}, err
	}
	for _, roleCode := range roleCodes {
		exists, err := s.roleExists(ctx, roleCode)
		if err != nil {
			return AssignUserRolesResult{}, err
		}
		if !exists {
			return AssignUserRolesResult{}, fmt.Errorf("%w: %s", ErrRoleNotFound, roleCode)
		}
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Table("users").Where("id = ? AND deleted_at IS NULL", targetUserID).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount == 0 {
			return ErrUserNotFound
		}
		userSubject := strconv.FormatInt(targetUserID, 10)
		if err := tx.Where("ptype = ? AND v0 = ?", "g", userSubject).Delete(&casbinRuleRecord{}).Error; err != nil {
			return err
		}
		for _, roleCode := range roleCodes {
			if err := tx.Create(&casbinRuleRecord{
				PType: "g",
				V0:    userSubject,
				V1:    roleCode,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return AssignUserRolesResult{}, err
	}
	s.publishChanged(ctx, []int64{targetUserID})
	return AssignUserRolesResult{UserID: targetUserID, RoleCodes: roleCodes, RoleNames: s.roleNamesForCodes(ctx, roleCodes)}, nil
}

func (s *RBACService) ensureSchema(ctx context.Context) error {
	if err := s.db.WithContext(ctx).AutoMigrate(&adminRoleRecord{}); err != nil {
		return err
	}
	for roleCode, roleName := range s.catalog.defaultRoleNames {
		if err := s.db.WithContext(ctx).Table("admin_roles").Where("role_code = ?", roleCode).
			Assign(map[string]any{
				"role_name":  roleName,
				"is_system":  true,
				"updated_at": gorm.Expr("NOW()"),
			}).
			FirstOrCreate(map[string]any{
				"role_code":  roleCode,
				"role_name":  roleName,
				"is_system":  true,
				"created_at": gorm.Expr("NOW()"),
				"updated_at": gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *RBACService) listRoles(ctx context.Context) ([]RoleItem, error) {
	roleMap := map[string]*RoleItem{}
	for roleCode, roleName := range s.catalog.defaultRoleNames {
		roleMap[roleCode] = &RoleItem{RoleCode: roleCode, RoleName: roleName, IsSystem: true, Permissions: []string{}}
	}

	type roleRow struct {
		RoleCode  string
		RoleName  string
		IsSystem  bool
		CreatedAt string
	}
	var roleRows []roleRow
	if err := s.db.WithContext(ctx).Table("admin_roles").
		Select("role_code, role_name, is_system, CAST(created_at AS text) AS created_at").
		Order("is_system DESC, role_code ASC").
		Scan(&roleRows).Error; err != nil {
		return nil, err
	}
	for _, row := range roleRows {
		roleCode := normalizeRoleCode(row.RoleCode)
		roleName := strings.TrimSpace(row.RoleName)
		if roleName == "" {
			roleName = s.roleName(roleCode)
		}
		roleMap[roleCode] = &RoleItem{RoleCode: roleCode, RoleName: roleName, IsSystem: row.IsSystem || s.catalog.systemRoles[roleCode], Permissions: []string{}, CreatedAt: row.CreatedAt}
	}

	permissionSets, err := s.rolePermissionSets(ctx)
	if err != nil {
		return nil, err
	}
	for roleCode := range permissionSets {
		if _, ok := roleMap[roleCode]; !ok {
			roleMap[roleCode] = &RoleItem{RoleCode: roleCode, RoleName: s.roleName(roleCode), IsSystem: s.catalog.systemRoles[roleCode], Permissions: []string{}}
		}
	}
	userCounts, err := s.roleUserCounts(ctx)
	if err != nil {
		return nil, err
	}

	roles := make([]RoleItem, 0, len(roleMap))
	for roleCode, role := range roleMap {
		permissions := sortedSet(permissionSets[roleCode])
		role.Permissions = permissions
		role.PermissionCount = len(permissions)
		role.UserCount = userCounts[roleCode]
		roles = append(roles, *role)
	}
	sort.Slice(roles, func(i, j int) bool {
		if roles[i].IsSystem != roles[j].IsSystem {
			return roles[i].IsSystem
		}
		return roles[i].RoleCode < roles[j].RoleCode
	})
	return roles, nil
}

func (s *RBACService) listPermissions(ctx context.Context) ([]PermissionItem, error) {
	metaMap := map[string]PermissionMeta{}
	for code, meta := range s.catalog.permissions {
		metaMap[code] = meta
	}
	type row struct {
		Object string `gorm:"column:v1"`
		Action string `gorm:"column:v2"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Table("casbin_rule").Select("v1, v2").Where("ptype = ? AND v1 <> '' AND v2 <> ''", "p").Group("v1, v2").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		code := row.Object + "." + row.Action
		if _, ok := metaMap[code]; !ok {
			metaMap[code] = PermissionMeta{Name: code, Module: permissionModule(code)}
		}
	}
	items := make([]PermissionItem, 0, len(metaMap))
	for code, meta := range metaMap {
		items = append(items, PermissionItem{PermissionCode: code, PermissionName: meta.Name, Module: meta.Module, Description: meta.Description})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Module != items[j].Module {
			return items[i].Module < items[j].Module
		}
		return items[i].PermissionCode < items[j].PermissionCode
	})
	return items, nil
}

func (s *RBACService) roleExists(ctx context.Context, roleCode string) (bool, error) {
	roleCode = normalizeRoleCode(roleCode)
	if roleCode == "" {
		return false, nil
	}
	if s.catalog.systemRoles[roleCode] {
		return true, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Table("admin_roles").Where("role_code = ?", roleCode).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := s.db.WithContext(ctx).Table("casbin_rule").Where("(ptype = 'p' AND v0 = ?) OR (ptype = 'g' AND v1 = ?)", roleCode, roleCode).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *RBACService) knownPermissionCodes(ctx context.Context) (map[string]struct{}, error) {
	items, err := s.listPermissions(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item.PermissionCode] = struct{}{}
	}
	return set, nil
}

func (s *RBACService) rolePermissionSets(ctx context.Context) (map[string]map[string]struct{}, error) {
	type row struct {
		RoleCode string `gorm:"column:v0"`
		Object   string `gorm:"column:v1"`
		Action   string `gorm:"column:v2"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Table("casbin_rule").Select("v0, v1, v2").Where("ptype = ? AND v0 <> '' AND v1 <> '' AND v2 <> ''", "p").Group("v0, v1, v2").Scan(&rows).Error; err != nil {
		return nil, err
	}
	sets := map[string]map[string]struct{}{}
	for _, row := range rows {
		roleCode := normalizeRoleCode(row.RoleCode)
		if roleCode == "" {
			continue
		}
		if _, ok := sets[roleCode]; !ok {
			sets[roleCode] = map[string]struct{}{}
		}
		sets[roleCode][row.Object+"."+row.Action] = struct{}{}
	}
	return sets, nil
}

func (s *RBACService) roleUserCounts(ctx context.Context) (map[string]int64, error) {
	type row struct {
		RoleCode string `gorm:"column:v1"`
		Count    int64  `gorm:"column:count"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Table("casbin_rule").
		Select("v1, COUNT(DISTINCT v0) AS count").
		Where("ptype = ? AND v1 <> ''", "g").
		Group("v1").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[normalizeRoleCode(row.RoleCode)] = row.Count
	}
	return counts, nil
}

func (s *RBACService) userIDsForRole(ctx context.Context, roleCode string) ([]int64, error) {
	var subjects []string
	if err := s.db.WithContext(ctx).Table("casbin_rule").Where("ptype = ? AND v1 = ?", "g", roleCode).Distinct("v0").Pluck("v0", &subjects).Error; err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(subjects))
	for _, subject := range subjects {
		userID, err := strconv.ParseInt(subject, 10, 64)
		if err == nil && userID > 0 {
			userIDs = append(userIDs, userID)
		}
	}
	return userIDs, nil
}

func (s *RBACService) roleNamesForCodes(ctx context.Context, roleCodes []string) []string {
	roleNameMap := map[string]string{}
	for code, name := range s.catalog.defaultRoleNames {
		roleNameMap[code] = name
	}
	type row struct {
		RoleCode string
		RoleName string
	}
	var rows []row
	_ = s.db.WithContext(ctx).Table("admin_roles").Select("role_code, role_name").Scan(&rows).Error
	for _, row := range rows {
		if strings.TrimSpace(row.RoleName) != "" {
			roleNameMap[normalizeRoleCode(row.RoleCode)] = row.RoleName
		}
	}
	roleNames := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleNames = append(roleNames, firstNonEmpty(roleNameMap[roleCode], roleCode))
	}
	return roleNames
}

func (s *RBACService) publishChanged(ctx context.Context, userIDs []int64) {
	pipe := s.redis.Pipeline()
	seenUsers := map[int64]struct{}{}
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seenUsers[userID]; ok {
			continue
		}
		seenUsers[userID] = struct{}{}
		pipe.Del(ctx, "passport:authz:"+strconv.FormatInt(userID, 10))
	}
	channels := []string{"passport:casbin:reload", "casbin:reload"}
	if strings.TrimSpace(s.redisPrefix) != "" {
		channels = append(channels, s.redisPrefix+"casbin:reload")
	}
	seenChannels := map[string]struct{}{}
	for _, channel := range channels {
		if _, ok := seenChannels[channel]; ok {
			continue
		}
		seenChannels[channel] = struct{}{}
		pipe.Publish(ctx, channel, "reload")
	}
	if _, err := pipe.Exec(ctx); err != nil {
		logx.Errorf("publish rbac changed: %v", err)
	}
}

func (s *RBACService) roleName(roleCode string) string {
	return firstNonEmpty(s.catalog.defaultRoleNames[roleCode], roleCode)
}

func defaultRoleCatalog() RoleCatalog {
	defaultRoles := map[string]string{
		"user":       "普通用户",
		"admin":      "管理员",
		"Premium":    "青衿",
		"PremiumPro": "鸿儒",
	}
	return RoleCatalog{
		defaultRoleNames: defaultRoles,
		systemRoles: map[string]bool{
			"user":       true,
			"admin":      true,
			"Premium":    true,
			"PremiumPro": true,
		},
		permissions: map[string]PermissionMeta{
			"relation.read":                 {Name: "查看关系", Module: "relation", Description: "允许查看关系快照"},
			"relation.following.read":       {Name: "查看关注列表", Module: "relation", Description: "允许查看关注列表"},
			"relation.followers.read":       {Name: "查看粉丝列表", Module: "relation", Description: "允许查看粉丝列表"},
			"relation.follow.create":        {Name: "关注用户", Module: "relation", Description: "允许关注目标用户"},
			"relation.follow.delete":        {Name: "取消关注", Module: "relation", Description: "允许取消关注目标用户"},
			"relation.block.create":         {Name: "拉黑用户", Module: "relation", Description: "允许拉黑目标用户"},
			"relation.block.delete":         {Name: "取消拉黑", Module: "relation", Description: "允许取消拉黑目标用户"},
			"content.post.read":             {Name: "查看帖子", Module: "content", Description: "允许查看帖子详情"},
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
			"admin.notification.send":       {Name: "发送系统通知", Module: "admin", Description: "允许向指定用户或全站发送系统通知"},
			"admin.ad.review":               {Name: "审核广告", Module: "admin", Description: "允许审核帖子广告订单"},
			"admin.membership.read":         {Name: "查看会员", Module: "admin", Description: "允许查看会员详情与订单"},
			"admin.membership.manage":       {Name: "管理会员", Module: "admin", Description: "允许后台发放、续期、撤销会员"},
			"membership.premium.active":     {Name: "青衿有效", Module: "membership", Description: "青衿增值服务有效"},
			"membership.premium_pro.active": {Name: "鸿儒有效", Module: "membership", Description: "鸿儒增值服务有效"},
			"monetization.ads.reduced":      {Name: "轻度减广告", Module: "monetization", Description: "减少信息流广告展示"},
			"monetization.ads.free":         {Name: "免信息流广告", Module: "monetization", Description: "免除信息流广告展示"},
		},
	}
}

func normalizeRoleCode(roleCode string) string {
	return strings.TrimSpace(roleCode)
}

func isValidRoleCode(roleCode string) bool {
	if len(roleCode) < 2 || len(roleCode) > 64 {
		return false
	}
	for _, r := range roleCode {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func normalizeRoleCodes(roleCodes []string) ([]string, error) {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleCode = normalizeRoleCode(roleCode)
		if roleCode == "" {
			continue
		}
		if !isValidRoleCode(roleCode) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRole, roleCode)
		}
		if _, ok := seen[roleCode]; ok {
			continue
		}
		seen[roleCode] = struct{}{}
		normalized = append(normalized, roleCode)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizePermissionCodes(permissionCodes []string, known map[string]struct{}) ([]string, error) {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(permissionCodes))
	for _, permissionCode := range permissionCodes {
		permissionCode = strings.TrimSpace(permissionCode)
		if permissionCode == "" {
			continue
		}
		if _, _, err := splitPermissionCode(permissionCode); err != nil {
			return nil, err
		}
		if _, ok := known[permissionCode]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrPermissionUnknown, permissionCode)
		}
		if _, ok := seen[permissionCode]; ok {
			continue
		}
		seen[permissionCode] = struct{}{}
		normalized = append(normalized, permissionCode)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func splitPermissionCode(permissionCode string) (string, string, error) {
	permissionCode = strings.TrimSpace(permissionCode)
	lastDot := strings.LastIndex(permissionCode, ".")
	if permissionCode == "" || lastDot <= 0 || lastDot >= len(permissionCode)-1 {
		return "", "", fmt.Errorf("%w: %s", ErrPermissionUnknown, permissionCode)
	}
	return permissionCode[:lastDot], permissionCode[lastDot+1:], nil
}

func permissionModule(permissionCode string) string {
	if i := strings.Index(permissionCode, "."); i > 0 {
		return permissionCode[:i]
	}
	return ""
}

func sortedSet(set map[string]struct{}) []string {
	if len(set) == 0 {
		return []string{}
	}
	items := make([]string, 0, len(set))
	for item := range set {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
