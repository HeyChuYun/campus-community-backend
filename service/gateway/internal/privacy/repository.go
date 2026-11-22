package privacy

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	BlacklistStatus(ctx context.Context, userID, targetUserID int64) (BlacklistStatus, error)
	ReplaceBlacklistScopes(ctx context.Context, userID, targetUserID int64, scopes []string) error
	IsScopedBlocked(ctx context.Context, leftUserID, rightUserID int64, scope string) (bool, error)
	UpsertPostSettings(ctx context.Context, postID int64, allowComment bool, hiddenUserIDs []int64) error
	PostSettings(ctx context.Context, postID int64) (PostSettings, error)
	VisiblePostAuthorID(ctx context.Context, postID int64) (int64, bool, error)
	IsViewerHiddenForPost(ctx context.Context, postID, viewerUserID int64) (bool, error)
}

type GormRepository struct {
	db *gorm.DB
}

type userPrivacyBlock struct {
	UserID       int64  `gorm:"column:user_id;primaryKey"`
	TargetUserID int64  `gorm:"column:target_user_id;primaryKey"`
	Scope        string `gorm:"column:scope;primaryKey"`
}

func (userPrivacyBlock) TableName() string {
	return "user_privacy_blocks"
}

type postPrivacySetting struct {
	PostID       int64 `gorm:"column:post_id;primaryKey"`
	AllowComment bool  `gorm:"column:allow_comment"`
}

func (postPrivacySetting) TableName() string {
	return "post_privacy_settings"
}

type postHiddenUser struct {
	PostID       int64 `gorm:"column:post_id;primaryKey"`
	HiddenUserID int64 `gorm:"column:hidden_user_id;primaryKey"`
}

func (postHiddenUser) TableName() string {
	return "post_hidden_users"
}

type visiblePost struct {
	ID           int64          `gorm:"column:id"`
	AuthorUserID int64          `gorm:"column:author_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (visiblePost) TableName() string {
	return "posts"
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: requireGormDB(db)}
}

func (r *GormRepository) BlacklistStatus(ctx context.Context, userID, targetUserID int64) (BlacklistStatus, error) {
	var blocks []userPrivacyBlock
	if err := r.db.WithContext(ctx).
		Select("scope").
		Where("user_id = ? AND target_user_id = ?", userID, targetUserID).
		Find(&blocks).Error; err != nil {
		return BlacklistStatus{}, err
	}

	var status BlacklistStatus
	for _, block := range blocks {
		switch strings.ToLower(strings.TrimSpace(block.Scope)) {
		case ScopeDM:
			status.DMBlocked = true
		case ScopePost:
			status.PostBlocked = true
		}
	}
	return status, nil
}

func (r *GormRepository) ReplaceBlacklistScopes(ctx context.Context, userID, targetUserID int64, scopes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND target_user_id = ?", userID, targetUserID).
			Delete(&userPrivacyBlock{}).Error; err != nil {
			return err
		}

		blocks := make([]userPrivacyBlock, 0, len(scopes))
		for _, scope := range scopes {
			normalizedScope := strings.TrimSpace(scope)
			if normalizedScope == "" {
				continue
			}
			blocks = append(blocks, userPrivacyBlock{
				UserID:       userID,
				TargetUserID: targetUserID,
				Scope:        normalizedScope,
			})
		}
		if len(blocks) == 0 {
			return nil
		}
		if err := tx.Create(&blocks).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *GormRepository) IsScopedBlocked(ctx context.Context, leftUserID, rightUserID int64, scope string) (bool, error) {
	if leftUserID <= 0 || rightUserID <= 0 || leftUserID == rightUserID {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&userPrivacyBlock{}).
		Where("scope = ?", scope).
		Where("((user_id = ? AND target_user_id = ?) OR (user_id = ? AND target_user_id = ?))", leftUserID, rightUserID, rightUserID, leftUserID).
		Count(&count).Error
	return count > 0, err
}

func (r *GormRepository) UpsertPostSettings(ctx context.Context, postID int64, allowComment bool, hiddenUserIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"allow_comment"}),
		}).Create(&postPrivacySetting{
			PostID:       postID,
			AllowComment: allowComment,
		}).Error; err != nil {
			return err
		}

		if err := tx.Where("post_id = ?", postID).Delete(&postHiddenUser{}).Error; err != nil {
			return err
		}
		userIDs := dedupePositiveInt64s(hiddenUserIDs)
		hiddenUsers := make([]postHiddenUser, 0, len(userIDs))
		for _, userID := range userIDs {
			hiddenUsers = append(hiddenUsers, postHiddenUser{
				PostID:       postID,
				HiddenUserID: userID,
			})
		}
		if len(hiddenUsers) == 0 {
			return nil
		}
		if err := tx.Create(&hiddenUsers).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *GormRepository) PostSettings(ctx context.Context, postID int64) (PostSettings, error) {
	if postID <= 0 {
		return PostSettings{AllowComment: true}, nil
	}
	var row postPrivacySetting
	err := r.db.WithContext(ctx).First(&row, "post_id = ?", postID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PostSettings{AllowComment: true}, nil
	}
	if err != nil {
		return PostSettings{}, err
	}
	return PostSettings{AllowComment: row.AllowComment}, nil
}

func (r *GormRepository) VisiblePostAuthorID(ctx context.Context, postID int64) (int64, bool, error) {
	if postID <= 0 {
		return 0, false, nil
	}
	var post visiblePost
	err := r.db.WithContext(ctx).
		Select("author_user_id").
		First(&post, "id = ?", postID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return post.AuthorUserID, true, nil
}

func (r *GormRepository) IsViewerHiddenForPost(ctx context.Context, postID, viewerUserID int64) (bool, error) {
	if postID <= 0 || viewerUserID <= 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&postHiddenUser{}).
		Where("post_id = ? AND hidden_user_id = ?", postID, viewerUserID).
		Count(&count).Error
	return count > 0, err
}

func dedupePositiveInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("privacy: gorm db is required")
	}
	return db
}
