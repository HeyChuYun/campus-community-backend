package logic

import (
	"context"
	"errors"
	"time"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"gorm.io/gorm"
)

const accountRestrictionScopeLogin = "account_login"

func syncAccountRestrictionState(ctx context.Context, svcCtx *svc.ServiceContext, user repository.User) (repository.User, *time.Time, error) {
	if svcCtx == nil || svcCtx.GormDB == nil {
		return user, nil, nil
	}
	if user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_DEACTIVATED) {
		return user, nil, nil
	}

	endsAt, active, err := activeAccountRestrictionEndsAt(ctx, svcCtx, user.ID)
	if err != nil {
		return user, nil, err
	}

	desiredStatus := user.AccountStatus
	if active {
		desiredStatus = int32(passportpb.AccountStatus_ACCOUNT_STATUS_BLOCKED)
	} else if user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_BLOCKED) {
		desiredStatus = int32(passportpb.AccountStatus_ACCOUNT_STATUS_NORMAL)
	}
	if desiredStatus != user.AccountStatus {
		user.AccountStatus = desiredStatus
		if err := svcCtx.UserStore.Save(user); err != nil {
			return user, endsAt, err
		}
	}
	return user, endsAt, nil
}

func activeAccountRestrictionEndsAt(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) (*time.Time, bool, error) {
	type row struct {
		EndsAt *time.Time `gorm:"column:ends_at"`
	}
	var item row
	now := time.Now()
	err := svcCtx.GormDB.WithContext(ctx).
		Table("user_restrictions").
		Select("ends_at").
		Where("user_id = ? AND scope = ? AND status = ?", userID, accountRestrictionScopeLogin, "active").
		Where("starts_at <= ?", now).
		Where("ends_at IS NULL OR ends_at > ?", now).
		Order("ends_at DESC NULLS LAST, id DESC").
		Take(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if item.EndsAt == nil {
		return nil, true, nil
	}
	value := *item.EndsAt
	return &value, true, nil
}
