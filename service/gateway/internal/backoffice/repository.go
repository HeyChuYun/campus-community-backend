package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	accountStatusNormal = 1
	accountStatusBanned = 3
)

type Repository interface {
	CountUsers(ctx context.Context) (int64, error)
	CountNewUsersSince(ctx context.Context, since time.Time) (int64, error)
	ListUsers(ctx context.Context, filter UserListFilter) (UserListResult, error)
	UpdateUserStatus(ctx context.Context, userID int64, status int) error
	UpsertUserRestriction(ctx context.Context, input UserRestrictionInput) error
}

type GormRepository struct {
	db *gorm.DB
}

type userRow struct {
	ID              int64
	Account         string
	Nickname        string
	AvatarURL       string       `gorm:"column:avatar_url"`
	AccountStatus   int          `gorm:"column:account_status"`
	PostMuteUntil   sql.NullTime `gorm:"column:post_mute_until"`
	DMMuteUntil     sql.NullTime `gorm:"column:dm_mute_until"`
	AccountBanUntil sql.NullTime `gorm:"column:account_ban_until"`
	ViolationCount  int64        `gorm:"column:violation_count"`
	CreatedAt       time.Time
}

type userRestrictionRecord struct {
	ID             int64         `gorm:"column:id;primaryKey"`
	RestrictionNo  string        `gorm:"column:restriction_no"`
	UserID         int64         `gorm:"column:user_id"`
	Scope          string        `gorm:"column:scope"`
	Status         string        `gorm:"column:status"`
	Source         string        `gorm:"column:source"`
	Reason         string        `gorm:"column:reason"`
	OperatorUserID sql.NullInt64 `gorm:"column:operator_user_id"`
	StartsAt       time.Time     `gorm:"column:starts_at"`
	EndsAt         sql.NullTime  `gorm:"column:ends_at"`
	UpdatedAt      time.Time     `gorm:"column:updated_at"`
	CreatedAt      time.Time     `gorm:"column:created_at"`
}

func (userRestrictionRecord) TableName() string {
	return "user_restrictions"
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: requireGormDB(db)}
}

func (r *GormRepository) CountUsers(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Table("users").Where("deleted_at IS NULL").Count(&total).Error
	return total, err
}

func (r *GormRepository) CountNewUsersSince(ctx context.Context, since time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Table("users").
		Where("created_at >= ? AND deleted_at IS NULL", since).
		Count(&total).Error
	return total, err
}

func (r *GormRepository) ListUsers(ctx context.Context, filter UserListFilter) (UserListResult, error) {
	normalizeUserListFilter(&filter)

	query := r.db.WithContext(ctx).Table("users").Where("deleted_at IS NULL")
	if strings.TrimSpace(filter.Search) != "" {
		like := "%" + strings.TrimSpace(filter.Search) + "%"
		query = query.Where("(account ILIKE ? OR nickname ILIKE ?)", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return UserListResult{}, err
	}

	var rows []userRow
	if err := query.
		Select(`
id,
account,
nickname,
COALESCE(avatar_url,'') AS avatar_url,
account_status,
(
	select ends_at
	from user_restrictions ur
	where ur.user_id = users.id
	  and ur.scope = 'post_publish'
	  and ur.status = 'active'
	  and (ur.ends_at is null or ur.ends_at > now())
	order by ur.ends_at desc nulls last, ur.id desc
	limit 1
) AS post_mute_until,
(
	select ends_at
	from user_restrictions ur
	where ur.user_id = users.id
	  and ur.scope = 'direct_message'
	  and ur.status = 'active'
	  and (ur.ends_at is null or ur.ends_at > now())
	order by ur.ends_at desc nulls last, ur.id desc
	limit 1
) AS dm_mute_until,
(
	select ends_at
	from user_restrictions ur
	where ur.user_id = users.id
	  and ur.scope = 'account_login'
	  and ur.status = 'active'
	  and (ur.ends_at is null or ur.ends_at > now())
	order by ur.ends_at desc nulls last, ur.id desc
	limit 1
) AS account_ban_until,
(
	select count(*)
	from content_violation_records vr
	where vr.user_id = users.id
) AS violation_count,
created_at`).
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&rows).Error; err != nil {
		return UserListResult{}, err
	}

	users := make([]UserItem, 0, len(rows))
	userIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		users = append(users, UserItem{
			ID:              row.ID,
			Account:         row.Account,
			Nickname:        row.Nickname,
			AvatarURL:       row.AvatarURL,
			AccountStatus:   row.AccountStatus,
			PostMuteUntil:   nullTimePointer(row.PostMuteUntil),
			DMMuteUntil:     nullTimePointer(row.DMMuteUntil),
			AccountBanUntil: nullTimePointer(row.AccountBanUntil),
			ViolationCount:  row.ViolationCount,
			CreatedAt:       row.CreatedAt,
			Roles:           []string{},
			RoleNames:       []string{},
		})
		userIDs = append(userIDs, row.ID)
	}

	roleCodesByUser, roleNamesByUser, err := r.loadUserRoleMaps(ctx, userIDs)
	if err != nil {
		return UserListResult{}, err
	}
	for i := range users {
		if users[i].AccountStatus == accountStatusBanned && users[i].AccountBanUntil == nil {
			users[i].AccountStatus = accountStatusNormal
		}
		users[i].Roles = roleCodesByUser[users[i].ID]
		users[i].RoleNames = roleNamesByUser[users[i].ID]
		if len(users[i].Roles) == 0 {
			users[i].Roles = []string{"user"}
		}
		if len(users[i].RoleNames) == 0 {
			users[i].RoleNames = []string{"普通用户"}
		}
	}

	return UserListResult{Users: users, Total: total}, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func nullInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func nullTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: true}
}

func nullIfZero(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (r *GormRepository) UpdateUserStatus(ctx context.Context, userID int64, status int) error {
	return r.db.WithContext(ctx).
		Table("users").
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]any{
			"account_status": status,
			"updated_at":     gorm.Expr("NOW()"),
		}).Error
}

func (r *GormRepository) UpsertUserRestriction(ctx context.Context, input UserRestrictionInput) error {
	now := time.Now()
	endsAt := now.Add(input.Duration)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing userRestrictionRecord
		err := tx.
			Select("id", "ends_at").
			Where("user_id = ? AND scope = ? AND status = ?", input.UserID, input.Scope, "active").
			Where("ends_at IS NULL OR ends_at > ?", now).
			Order("ends_at DESC NULLS LAST, id DESC").
			Take(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing.EndsAt.Valid && existing.EndsAt.Time.After(endsAt) {
			endsAt = existing.EndsAt.Time
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if createErr := tx.Create(&userRestrictionRecord{
				RestrictionNo:  buildRestrictionNo(input.UserID, input.Scope, now),
				UserID:         input.UserID,
				Scope:          input.Scope,
				Status:         "active",
				Source:         "admin_manual",
				Reason:         input.Reason,
				OperatorUserID: nullInt64(input.OperatorUserID),
				StartsAt:       now,
				EndsAt:         nullTime(endsAt),
				CreatedAt:      now,
				UpdatedAt:      now,
			}).Error; createErr != nil {
				return createErr
			}
		} else {
			if updateErr := tx.Model(&userRestrictionRecord{}).
				Where("id = ?", existing.ID).
				Updates(map[string]any{
					"source":           "admin_manual",
					"reason":           input.Reason,
					"operator_user_id": nullIfZero(input.OperatorUserID),
					"ends_at":          endsAt,
					"updated_at":       gorm.Expr("NOW()"),
				}).Error; updateErr != nil {
				return updateErr
			}
		}
		if input.Scope == UserRestrictionScopeAccountLogin {
			return tx.Table("users").
				Where("id = ? AND deleted_at IS NULL", input.UserID).
				Updates(map[string]any{
					"account_status": accountStatusBanned,
					"updated_at":     gorm.Expr("NOW()"),
				}).Error
		}
		return nil
	})
}

func (r *GormRepository) loadUserRoleMaps(ctx context.Context, userIDs []int64) (map[int64][]string, map[int64][]string, error) {
	roleCodesByUser := make(map[int64][]string, len(userIDs))
	roleNamesByUser := make(map[int64][]string, len(userIDs))
	if len(userIDs) == 0 {
		return roleCodesByUser, roleNamesByUser, nil
	}

	subjects := make([]string, 0, len(userIDs))
	subjectToUserID := make(map[string]int64, len(userIDs))
	for _, userID := range userIDs {
		subject := fmt.Sprintf("%d", userID)
		subjects = append(subjects, subject)
		subjectToUserID[subject] = userID
	}

	type roleRow struct {
		Subject  string `gorm:"column:v0"`
		RoleCode string `gorm:"column:v1"`
		RoleName string `gorm:"column:role_name"`
	}
	var rows []roleRow
	if err := r.db.WithContext(ctx).
		Table("casbin_rule AS c").
		Select("c.v0, c.v1, COALESCE(ar.role_name, c.v1) AS role_name").
		Joins("LEFT JOIN admin_roles ar ON ar.role_code = c.v1").
		Where("c.ptype = ? AND c.v0 IN ?", "g", subjects).
		Order("c.v1 ASC").
		Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	for _, row := range rows {
		userID, ok := subjectToUserID[row.Subject]
		if !ok {
			continue
		}
		roleCodesByUser[userID] = append(roleCodesByUser[userID], row.RoleCode)
		roleNamesByUser[userID] = append(roleNamesByUser[userID], row.RoleName)
	}
	return roleCodesByUser, roleNamesByUser, nil
}

func normalizeUserListFilter(filter *UserListFilter) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("backoffice: gorm db is required")
	}
	return db
}

func buildRestrictionNo(userID int64, scope string, now time.Time) string {
	return fmt.Sprintf("adm_rst_%d_%d_%s", userID, now.UnixNano(), strings.TrimSpace(scope))
}
