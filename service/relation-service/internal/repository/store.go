package repository

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const activeFollowStatus int32 = 1
const inactiveFollowStatus int32 = 2

type RelationSnapshot struct {
	ActorUserID    int64
	TargetUserID   int64
	IsFollowing    bool
	IsFollowedBy   bool
	IsMutualFollow bool
	IsFriend       bool
	IsBlocking     bool
	IsBlockedBy    bool
}

type UserCounter struct {
	UserID         int64
	FollowingCount int64
	FollowerCount  int64
	FriendCount    int64
	BlockedCount   int64
}

type RelatedUserItem struct {
	UserID        int64
	RelatedUserID int64
	Cursor        string
}

type RelationStore interface {
	Follow(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, UserCounter, error)
	Unfollow(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, UserCounter, error)
	Block(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, error)
	Unblock(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, error)
	GetRelation(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, error)
	BatchGetRelation(ctx context.Context, actorUserID int64, targetUserIDs []int64) ([]RelationSnapshot, error)
	ListFollowing(ctx context.Context, userID, page, pageSize int64) ([]RelatedUserItem, bool, error)
	ListFollowers(ctx context.Context, userID, page, pageSize int64) ([]RelatedUserItem, bool, error)
	GetCounter(ctx context.Context, userID int64) (UserCounter, error)
}

type UserFollow struct {
	ID             int64     `gorm:"column:id;primaryKey"`
	FollowerUserID int64     `gorm:"column:follower_user_id"`
	FolloweeUserID int64     `gorm:"column:followee_user_id"`
	Status         int32     `gorm:"column:status"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (UserFollow) TableName() string { return "user_follows" }

type UserBlock struct {
	UserID        int64     `gorm:"column:user_id;primaryKey"`
	BlockedUserID int64     `gorm:"column:blocked_user_id;primaryKey"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (UserBlock) TableName() string { return "user_blocks" }

type relationStore struct {
	db *gorm.DB
}

func NewRelationStore(db *gorm.DB) RelationStore {
	return &relationStore{db: requireGormDB(db)}
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("relation repository: gorm db is required")
	}
	return db
}

func (s *relationStore) Follow(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, UserCounter, error) {
	now := time.Now()
	follow := UserFollow{
		FollowerUserID: actorUserID,
		FolloweeUserID: targetUserID,
		Status:         activeFollowStatus,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "follower_user_id"}, {Name: "followee_user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":     activeFollowStatus,
			"updated_at": now,
		}),
	}).Create(&follow).Error
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, UserCounter{}, err
	}
	return s.snapshotWithCounters(ctx, actorUserID, targetUserID)
}

func (s *relationStore) Unfollow(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, UserCounter, error) {
	err := s.db.WithContext(ctx).Model(&UserFollow{}).
		Where("follower_user_id = ? and followee_user_id = ?", actorUserID, targetUserID).
		Updates(map[string]any{"status": inactiveFollowStatus, "updated_at": time.Now()}).
		Error
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, UserCounter{}, err
	}
	return s.snapshotWithCounters(ctx, actorUserID, targetUserID)
}

func (s *relationStore) Block(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&UserBlock{
			UserID:        actorUserID,
			BlockedUserID: targetUserID,
			CreatedAt:     time.Now(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&UserFollow{}).
			Where("(follower_user_id = ? and followee_user_id = ?) or (follower_user_id = ? and followee_user_id = ?)",
				actorUserID, targetUserID, targetUserID, actorUserID).
			Updates(map[string]any{"status": inactiveFollowStatus, "updated_at": time.Now()}).
			Error
	})
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, err
	}
	snapshot, err := s.GetRelation(ctx, actorUserID, targetUserID)
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, err
	}
	counter, err := s.getCounter(ctx, actorUserID)
	return snapshot, counter, err
}

func (s *relationStore) Unblock(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, error) {
	if err := s.db.WithContext(ctx).Where("user_id = ? and blocked_user_id = ?", actorUserID, targetUserID).Delete(&UserBlock{}).Error; err != nil {
		return RelationSnapshot{}, UserCounter{}, err
	}
	snapshot, err := s.GetRelation(ctx, actorUserID, targetUserID)
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, err
	}
	counter, err := s.getCounter(ctx, actorUserID)
	return snapshot, counter, err
}

func (s *relationStore) GetRelation(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, error) {
	snapshot := RelationSnapshot{
		ActorUserID:  actorUserID,
		TargetUserID: targetUserID,
	}
	var err error
	snapshot.IsFollowing, err = s.existsFollow(ctx, actorUserID, targetUserID)
	if err != nil {
		return RelationSnapshot{}, err
	}
	snapshot.IsFollowedBy, err = s.existsFollow(ctx, targetUserID, actorUserID)
	if err != nil {
		return RelationSnapshot{}, err
	}
	snapshot.IsBlocking, err = s.existsBlock(ctx, actorUserID, targetUserID)
	if err != nil {
		return RelationSnapshot{}, err
	}
	snapshot.IsBlockedBy, err = s.existsBlock(ctx, targetUserID, actorUserID)
	if err != nil {
		return RelationSnapshot{}, err
	}
	snapshot.IsMutualFollow = snapshot.IsFollowing && snapshot.IsFollowedBy
	snapshot.IsFriend = snapshot.IsMutualFollow && !snapshot.IsBlocking && !snapshot.IsBlockedBy
	return snapshot, nil
}

func (s *relationStore) BatchGetRelation(ctx context.Context, actorUserID int64, targetUserIDs []int64) ([]RelationSnapshot, error) {
	uniqueTargets := uniqueInt64(targetUserIDs)
	snapshots := make([]RelationSnapshot, 0, len(uniqueTargets))
	for _, targetUserID := range uniqueTargets {
		snapshot, err := s.GetRelation(ctx, actorUserID, targetUserID)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func (s *relationStore) ListFollowing(ctx context.Context, userID, page, pageSize int64) ([]RelatedUserItem, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	var rows []UserFollow
	if err := s.db.WithContext(ctx).Where("follower_user_id = ? and status = ?", userID, activeFollowStatus).
		Order("id desc").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Find(&rows).Error; err != nil {
		return nil, false, err
	}
	return followRowsToRelatedItems(rows, userID, false, limit), int64(len(rows)) > limit, nil
}

func (s *relationStore) ListFollowers(ctx context.Context, userID, page, pageSize int64) ([]RelatedUserItem, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	var rows []UserFollow
	if err := s.db.WithContext(ctx).Where("followee_user_id = ? and status = ?", userID, activeFollowStatus).
		Order("id desc").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Find(&rows).Error; err != nil {
		return nil, false, err
	}
	return followRowsToRelatedItems(rows, userID, true, limit), int64(len(rows)) > limit, nil
}

func (s *relationStore) GetCounter(ctx context.Context, userID int64) (UserCounter, error) {
	return s.getCounter(ctx, userID)
}

func (s *relationStore) snapshotWithCounters(ctx context.Context, actorUserID, targetUserID int64) (RelationSnapshot, UserCounter, UserCounter, error) {
	snapshot, err := s.GetRelation(ctx, actorUserID, targetUserID)
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, UserCounter{}, err
	}
	actorCounter, err := s.getCounter(ctx, actorUserID)
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, UserCounter{}, err
	}
	targetCounter, err := s.getCounter(ctx, targetUserID)
	if err != nil {
		return RelationSnapshot{}, UserCounter{}, UserCounter{}, err
	}
	return snapshot, actorCounter, targetCounter, nil
}

func (s *relationStore) getCounter(ctx context.Context, userID int64) (UserCounter, error) {
	counter := UserCounter{UserID: userID}
	if err := s.db.WithContext(ctx).Model(&UserFollow{}).Where("follower_user_id = ? and status = ?", userID, activeFollowStatus).Count(&counter.FollowingCount).Error; err != nil {
		return UserCounter{}, err
	}
	if err := s.db.WithContext(ctx).Model(&UserFollow{}).Where("followee_user_id = ? and status = ?", userID, activeFollowStatus).Count(&counter.FollowerCount).Error; err != nil {
		return UserCounter{}, err
	}
	if err := s.db.WithContext(ctx).Model(&UserBlock{}).Where("user_id = ?", userID).Count(&counter.BlockedCount).Error; err != nil {
		return UserCounter{}, err
	}
	err := s.db.WithContext(ctx).
		Table("user_follows AS uf").
		Joins("JOIN user_follows AS uf2 ON uf2.follower_user_id = uf.followee_user_id AND uf2.followee_user_id = ? AND uf2.status = ?", userID, activeFollowStatus).
		Where("uf.follower_user_id = ? AND uf.status = ?", userID, activeFollowStatus).
		Count(&counter.FriendCount).Error
	if err != nil {
		return UserCounter{}, err
	}
	return counter, nil
}

func (s *relationStore) existsFollow(ctx context.Context, actorUserID, targetUserID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&UserFollow{}).
		Where("follower_user_id = ? and followee_user_id = ? and status = ?", actorUserID, targetUserID, activeFollowStatus).
		Count(&count).Error
	return count > 0, err
}

func (s *relationStore) existsBlock(ctx context.Context, actorUserID, targetUserID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&UserBlock{}).
		Where("user_id = ? and blocked_user_id = ?", actorUserID, targetUserID).
		Count(&count).Error
	return count > 0, err
}

func followRowsToRelatedItems(rows []UserFollow, userID int64, followerMode bool, limit int64) []RelatedUserItem {
	maxItems := len(rows)
	if int64(maxItems) > limit {
		maxItems = int(limit)
	}
	items := make([]RelatedUserItem, 0, maxItems)
	for index := 0; index < maxItems; index++ {
		row := rows[index]
		relatedUserID := row.FolloweeUserID
		if followerMode {
			relatedUserID = row.FollowerUserID
		}
		items = append(items, RelatedUserItem{
			UserID:        userID,
			RelatedUserID: relatedUserID,
			Cursor:        strconv.FormatInt(row.ID, 10),
		})
	}
	return items
}

func normalizePage(page, pageSize int64) (int64, int64, int64, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	return page, pageSize, offset, pageSize
}

func uniqueInt64(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
