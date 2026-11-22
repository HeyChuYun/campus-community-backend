package commercial

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MembershipDetail struct {
	UserID        int64
	UserNickname  sql.NullString
	PlanCode      string
	StartsAt      time.Time
	EndsAt        time.Time
	SourceOrderNo string
	UpdatedAt     time.Time
}

type MembershipAdminOperation struct {
	ID               int64      `gorm:"column:id;primaryKey"`
	OperationNo      string     `gorm:"column:operation_no;uniqueIndex"`
	UserID           int64      `gorm:"column:user_id;index:idx_membership_admin_ops_user_created,priority:1"`
	OperatorUserID   int64      `gorm:"column:operator_user_id;index"`
	Action           string     `gorm:"column:action"`
	PlanCode         string     `gorm:"column:plan_code"`
	DurationValue    int        `gorm:"column:duration_value"`
	DurationUnit     string     `gorm:"column:duration_unit"`
	PreviousPlanCode string     `gorm:"column:previous_plan_code"`
	PreviousStartsAt *time.Time `gorm:"column:previous_starts_at"`
	PreviousEndsAt   *time.Time `gorm:"column:previous_ends_at"`
	NewStartsAt      *time.Time `gorm:"column:new_starts_at"`
	NewEndsAt        *time.Time `gorm:"column:new_ends_at"`
	Reason           string     `gorm:"column:reason;type:text"`
	CreatedAt        time.Time  `gorm:"column:created_at;index:idx_membership_admin_ops_user_created,priority:2,sort:desc"`
}

func (MembershipAdminOperation) TableName() string { return "membership_admin_operations" }

type MembershipAdminOperationView struct {
	OperationNo      string
	UserID           int64
	OperatorUserID   int64
	Action           string
	PlanCode         string
	DurationValue    int
	DurationUnit     string
	PreviousPlanCode string
	PreviousStartsAt sql.NullTime
	PreviousEndsAt   sql.NullTime
	NewStartsAt      sql.NullTime
	NewEndsAt        sql.NullTime
	Reason           string
	CreatedAt        time.Time
}

type MembershipMutationInput struct {
	TargetUserID   int64
	OperatorUserID int64
	PlanCode       string
	DurationValue  int
	DurationUnit   string
	Reason         string
}

type MembershipMutationResult struct {
	Action           string
	OperationNo      string
	UserID           int64
	PreviousPlanCode string
	PreviousStartsAt *time.Time
	PreviousEndsAt   *time.Time
	CurrentPlanCode  string
	CurrentStartsAt  *time.Time
	CurrentEndsAt    *time.Time
}

func (r *gormRepository) GetMembershipDetail(ctx context.Context, userID int64) (MembershipDetail, error) {
	if err := r.ensureMembershipAdminSchema(ctx); err != nil {
		return MembershipDetail{}, err
	}
	var detail MembershipDetail
	err := r.db.WithContext(ctx).
		Table("user_memberships um").
		Select("um.user_id, u.nickname as user_nickname, um.plan_code, um.starts_at, um.ends_at, um.source_order_no, um.updated_at").
		Joins("join users u on u.id = um.user_id").
		Where("um.user_id = ?", userID).
		First(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MembershipDetail{}, ErrMembershipNotFound
	}
	return detail, err
}

func (r *gormRepository) ListMembershipAdminOperations(ctx context.Context, userID int64, limit int) ([]MembershipAdminOperationView, error) {
	if err := r.ensureMembershipAdminSchema(ctx); err != nil {
		return nil, err
	}
	var items []MembershipAdminOperationView
	err := r.db.WithContext(ctx).
		Table("membership_admin_operations").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *gormRepository) GrantMembership(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string) (MembershipMutationResult, error) {
	if err := r.ensureMembershipAdminSchema(ctx); err != nil {
		return MembershipMutationResult{}, err
	}
	return r.applyMembershipMutation(ctx, input, managedRoleCodes, AdminMembershipActionGrant, func(now time.Time, current *UserMembership) (membershipState, error) {
		startsAt := now
		recordStartsAt := now
		if current != nil && current.PlanCode == input.PlanCode && current.EndsAt.After(now) {
			startsAt = current.EndsAt
			recordStartsAt = current.StartsAt
		}
		endsAt, err := AddMembershipDuration(startsAt, input.DurationUnit, input.DurationValue)
		if err != nil {
			return membershipState{}, err
		}
		return membershipState{
			PlanCode: input.PlanCode,
			StartsAt: recordStartsAt,
			EndsAt:   endsAt,
		}, nil
	})
}

func (r *gormRepository) ExtendMembership(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string) (MembershipMutationResult, error) {
	if err := r.ensureMembershipAdminSchema(ctx); err != nil {
		return MembershipMutationResult{}, err
	}
	return r.applyMembershipMutation(ctx, input, managedRoleCodes, AdminMembershipActionExtend, func(now time.Time, current *UserMembership) (membershipState, error) {
		if current == nil {
			return membershipState{}, ErrMembershipNotFound
		}
		planCode := current.PlanCode
		if strings.TrimSpace(input.PlanCode) != "" && input.PlanCode != current.PlanCode {
			return membershipState{}, NewError("INVALID_PLAN_TRANSITION", "续期只能延长当前会员方案", 400)
		}
		base := now
		startsAt := now
		if current.EndsAt.After(now) {
			base = current.EndsAt
			startsAt = current.StartsAt
		}
		endsAt, err := AddMembershipDuration(base, input.DurationUnit, input.DurationValue)
		if err != nil {
			return membershipState{}, err
		}
		return membershipState{
			PlanCode: planCode,
			StartsAt: startsAt,
			EndsAt:   endsAt,
		}, nil
	})
}

func (r *gormRepository) RevokeMembership(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string) (MembershipMutationResult, error) {
	if err := r.ensureMembershipAdminSchema(ctx); err != nil {
		return MembershipMutationResult{}, err
	}
	return r.applyMembershipMutation(ctx, input, managedRoleCodes, AdminMembershipActionRevoke, func(time.Time, *UserMembership) (membershipState, error) {
		return membershipState{Revoked: true}, nil
	})
}

type membershipState struct {
	PlanCode string
	StartsAt time.Time
	EndsAt   time.Time
	Revoked  bool
}

type membershipMutationBuilder func(now time.Time, current *UserMembership) (membershipState, error)

func (r *gormRepository) applyMembershipMutation(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string, action string, builder membershipMutationBuilder) (MembershipMutationResult, error) {
	var result MembershipMutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.ensureTargetUser(tx, input.TargetUserID); err != nil {
			return err
		}
		if action != AdminMembershipActionRevoke {
			if err := r.ensurePlanActive(tx, input.PlanCode); err != nil {
				return err
			}
		}

		current, err := r.lockMembership(tx, input.TargetUserID)
		if err != nil {
			return err
		}
		now := time.Now()
		nextState, err := builder(now, current)
		if err != nil {
			return err
		}

		result = MembershipMutationResult{
			Action:      action,
			OperationNo: NewOrderNo("AM"),
			UserID:      input.TargetUserID,
		}
		if current != nil {
			result.PreviousPlanCode = current.PlanCode
			result.PreviousStartsAt = timePtr(current.StartsAt)
			result.PreviousEndsAt = timePtr(current.EndsAt)
		}

		if nextState.Revoked {
			if current == nil {
				return ErrMembershipNotFound
			}
			if err := tx.Where("user_id = ?", input.TargetUserID).Delete(&UserMembership{}).Error; err != nil {
				return err
			}
			if err := r.syncMembershipRoles(tx, input.TargetUserID, "", managedRoleCodes); err != nil {
				return err
			}
		} else {
			membership := UserMembership{
				UserID:        input.TargetUserID,
				PlanCode:      nextState.PlanCode,
				StartsAt:      nextState.StartsAt,
				EndsAt:        nextState.EndsAt,
				SourceOrderNo: result.OperationNo,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"plan_code":       membership.PlanCode,
					"starts_at":       membership.StartsAt,
					"ends_at":         membership.EndsAt,
					"source_order_no": membership.SourceOrderNo,
					"updated_at":      now,
				}),
			}).Create(&membership).Error; err != nil {
				return err
			}
			if err := r.syncMembershipRoles(tx, input.TargetUserID, membership.PlanCode, managedRoleCodes); err != nil {
				return err
			}
			result.CurrentPlanCode = membership.PlanCode
			result.CurrentStartsAt = timePtr(membership.StartsAt)
			result.CurrentEndsAt = timePtr(membership.EndsAt)
		}

		return tx.Create(&MembershipAdminOperation{
			OperationNo:      result.OperationNo,
			UserID:           input.TargetUserID,
			OperatorUserID:   input.OperatorUserID,
			Action:           action,
			PlanCode:         firstNonEmptyString(nextState.PlanCode, input.PlanCode),
			DurationValue:    input.DurationValue,
			DurationUnit:     input.DurationUnit,
			PreviousPlanCode: result.PreviousPlanCode,
			PreviousStartsAt: result.PreviousStartsAt,
			PreviousEndsAt:   result.PreviousEndsAt,
			NewStartsAt:      result.CurrentStartsAt,
			NewEndsAt:        result.CurrentEndsAt,
			Reason:           strings.TrimSpace(input.Reason),
			CreatedAt:        now,
		}).Error
	})
	return result, err
}

func (r *gormRepository) ensureMembershipAdminSchema(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&MembershipAdminOperation{})
}

func (r *gormRepository) ensureTargetUser(tx *gorm.DB, userID int64) error {
	if userID <= 0 {
		return ErrInvalidArgument
	}
	var count int64
	if err := tx.Table("users").Where("id = ? and deleted_at is null", userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return NewError("USER_NOT_FOUND", "目标用户不存在", 404)
	}
	return nil
}

func (r *gormRepository) ensurePlanActive(tx *gorm.DB, planCode string) error {
	var count int64
	if err := tx.Table("membership_plans").Where("plan_code = ? and is_active = true", planCode).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return NewError("INVALID_PLAN", "会员方案不存在或未启用", 400)
	}
	return nil
}

func (r *gormRepository) lockMembership(tx *gorm.DB, userID int64) (*UserMembership, error) {
	return queryMembershipForUpdate(tx, "user_id = ?", userID)
}

func (r *gormRepository) syncMembershipRoles(tx *gorm.DB, userID int64, planCode string, managedRoleCodes []string) error {
	userIDText := strconv.FormatInt(userID, 10)
	if len(managedRoleCodes) > 0 {
		if err := tx.Where("ptype = ? and v0 = ? and v1 in ?", "g", userIDText, managedRoleCodes).Delete(&CasbinRule{}).Error; err != nil {
			return err
		}
	}
	if strings.TrimSpace(planCode) == "" {
		return nil
	}
	return tx.Create(&CasbinRule{PType: "g", V0: userIDText, V1: planCode}).Error
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
