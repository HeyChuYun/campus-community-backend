package svc

import (
	"context"
	"errors"
	"strings"
	"time"

	"hfjj/backend/pkg/rpcx"
	"hfjj/backend/passport-service/internal/config"
	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/school-service/schoolservice"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	gozeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config             config.Config
	GormDB             *gorm.DB
	Redis              *redis.Client
	UserStore          repository.UserStore
	TokenStore         repository.TokenStore
	SessionStore       repository.SessionStore
	LoginStore         repository.LoginRecordStore
	BehaviorVerifier   repository.BehaviorVerifier
	AuthzStore         repository.AuthorizationSnapshotStore
	RoleChangeLogStore repository.UserRoleChangeLogStore
	MembershipExpiry   repository.MembershipExpiryStore
	Enforcer           *casbin.SyncedEnforcer
	SchoolRPC          schoolservice.SchoolService
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 学校服务客户端走项目兼容 resolver，继续使用 etcd 发现，不退回固定地址。
	schoolClient := schoolservice.NewSchoolService(rpcx.MustNewClient(c.SchoolRpc))
	gormDB := newGormPostgresClient(c.Postgres.DSN)
	rdb := newRedisClient(c.AppRedis)
	prefix := c.AppRedis.Key
	if prefix == "" {
		prefix = "passport:"
	}

	// tokenStore 同时承载 refresh_token 和登录会话状态。
	// 登录会话用于 JWT access_token 的撤销与刷新轮换校验。
	tokenStore := repository.NewTokenStore(rdb, prefix)
	sessionStore, _ := tokenStore.(repository.SessionStore)

	svcCtx := &ServiceContext{
		Config:             c,
		GormDB:             gormDB,
		Redis:              rdb,
		UserStore:          repository.NewUserStore(gormDB),
		TokenStore:         tokenStore,
		SessionStore:       sessionStore,
		LoginStore:         repository.NewLoginRecordStore(gormDB),
		BehaviorVerifier:   repository.NewBehaviorVerifier(c.GeeTest),
		AuthzStore:         repository.NewAuthorizationSnapshotStore(rdb, prefix),
		RoleChangeLogStore: repository.NewUserRoleChangeLogStore(gormDB),
		MembershipExpiry:   repository.NewMembershipExpiryStore(gormDB),
		Enforcer:           newCasbinEnforcer(gormDB),
		SchoolRPC:          schoolClient,
	}
	svcCtx.startCasbinReloadSubscriber(prefix)
	svcCtx.startMembershipExpiryReconciler()
	return svcCtx
}

// PublishCasbinPolicyReload 通知其他 passport-service 实例重新加载 Casbin 策略。
// 这样在多实例部署时，某个实例完成角色变更后，其他实例也能尽快看到新策略。
func (s *ServiceContext) PublishCasbinPolicyReload() error {
	if s.Redis == nil {
		return nil
	}

	channel := redisChannel(s.Config.AppRedis.Key, "casbin:reload")
	return s.Redis.Publish(context.Background(), channel, "reload").Err()
}

func (s *ServiceContext) startCasbinReloadSubscriber(prefix string) {
	if s.Redis == nil {
		return
	}

	channel := redisChannel(prefix, "casbin:reload")
	pubsub := s.Redis.Subscribe(context.Background(), channel)
	go func() {
		defer pubsub.Close()
		for range pubsub.Channel() {
			// 收到策略变更广播后，重新装载 Casbin 策略，保证多实例授权结果尽快一致。
			if err := s.Enforcer.LoadPolicy(); err != nil {
				logx.Errorf("reload casbin policy failed: %v", err)
			}
		}
	}()
}

func (s *ServiceContext) startMembershipExpiryReconciler() {
	if s.GormDB == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			s.reconcileExpiredMemberships()
			<-ticker.C
		}
	}()
}

func (s *ServiceContext) reconcileExpiredMemberships() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	userIDs, err := s.MembershipExpiry.ExpiredMembershipUserIDs(ctx)
	if err != nil {
		logx.Errorf("query expired memberships failed: %v", err)
		return
	}
	if len(userIDs) == 0 {
		return
	}

	if err = s.MembershipExpiry.RemoveExpiredMembershipsAndRoles(ctx, userIDs, membershipRoleCodes()); err != nil {
		logx.Errorf("commit expired membership tx failed: %v", err)
		return
	}

	for _, userID := range userIDs {
		_ = s.AuthzStore.Delete(userID)
	}
	if err = s.Enforcer.LoadPolicy(); err != nil {
		logx.Errorf("reload casbin after membership expiry failed: %v", err)
	}
	if err = s.PublishCasbinPolicyReload(); err != nil {
		logx.Errorf("publish membership expiry rbac reload failed: %v", err)
	}
}

func redisChannel(prefix, suffix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return suffix
	}
	return prefix + suffix
}

func membershipRoleCodes() []string {
	return []string{"Premium", "PremiumPro"}
}

func newGormPostgresClient(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}

func newRedisClient(cfg gozeroredis.RedisKeyConf) *redis.Client {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:      strings.TrimSpace(strings.Split(cfg.Host, ",")[0]),
		Username:  cfg.User,
		Password:  cfg.Pass,
		DB:        0,
		TLSConfig: nil,
	})
}

func newCasbinEnforcer(db *gorm.DB) *casbin.SyncedEnforcer {
	modelText := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

	var (
		enforcer *casbin.SyncedEnforcer
		err      error
	)
	mdl, err := model.NewModelFromString(modelText)
	if err != nil {
		panic(err)
	}

	if db == nil {
		enforcer, err = casbin.NewSyncedEnforcer(mdl)
	} else {
		adapter, aerr := gormadapter.NewAdapterByDB(db)
		if aerr != nil {
			panic(aerr)
		}
		enforcer, err = casbin.NewSyncedEnforcer(mdl, adapter)
	}
	if err != nil {
		panic(err)
	}

	enforcer.EnableAutoSave(true)
	if err := enforcer.LoadPolicy(); err != nil {
		panic(err)
	}
	if err := seedCasbinPolicies(enforcer); err != nil {
		panic(err)
	}
	return enforcer
}

func seedCasbinPolicies(enforcer *casbin.SyncedEnforcer) error {
	for roleCode, permissionCodes := range defaultCasbinRolePolicies() {
		for _, permissionCode := range permissionCodes {
			object, action, err := splitPermissionCode(permissionCode)
			if err != nil {
				return err
			}
			if _, err := enforcer.AddPolicy(roleCode, object, action); err != nil {
				return err
			}
		}
	}
	return nil
}

func defaultCasbinRolePolicies() map[string][]string {
	return map[string][]string{
		"user": {
			"relation.read",
			"relation.following.read",
			"relation.followers.read",
			"relation.follow.create",
			"relation.follow.delete",
			"relation.block.create",
			"relation.block.delete",
			"content.post.read",
			"content.feed.read",
			"content.post.create",
			"content.post.delete.own",
			"content.comment.create",
			"content.comment.delete.own",
			"content.like.create",
			"content.favorite.create",
			"message.send",
			"profile.update",
			"school.profile.read",
			"school.schedule.read",
			"school.grade.read",
		},
		"admin": {
			"relation.read",
			"relation.following.read",
			"relation.followers.read",
			"relation.follow.create",
			"relation.follow.delete",
			"relation.block.create",
			"relation.block.delete",
			"content.post.read",
			"content.feed.read",
			"content.post.create",
			"content.post.delete.own",
			"content.post.delete.any",
			"content.comment.create",
			"content.comment.delete.own",
			"content.comment.delete.any",
			"content.like.create",
			"content.favorite.create",
			"message.send",
			"profile.update",
			"school.profile.read",
			"school.schedule.read",
			"school.grade.read",
			"admin.user.read",
			"admin.user.role.assign",
			"admin.user.ban",
			"admin.post.review.read",
			"admin.post.review.manage",
			"admin.content.moderate",
			"admin.ad.review",
			"admin.membership.read",
			"admin.membership.manage",
		},
		"Premium": {
			"membership.premium.active",
			"monetization.ads.reduced",
		},
		"PremiumPro": {
			"membership.premium_pro.active",
			"monetization.ads.free",
		},
	}
}

func splitPermissionCode(permissionCode string) (string, string, error) {
	permissionCode = strings.TrimSpace(permissionCode)
	lastDot := strings.LastIndex(permissionCode, ".")
	if permissionCode == "" || lastDot <= 0 || lastDot >= len(permissionCode)-1 {
		return "", "", errors.New("invalid permission code")
	}
	return permissionCode[:lastDot], permissionCode[lastDot+1:], nil
}
