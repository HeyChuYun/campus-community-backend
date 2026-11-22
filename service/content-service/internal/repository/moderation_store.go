package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"hfjj/backend/proto/contentpb"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RestrictionScopePostPublish   = "post_publish"
	RestrictionScopeDirectMessage = "direct_message"

	RestrictionStatusActive  = "active"
	RestrictionStatusRevoked = "revoked"
	RestrictionStatusExpired = "expired"
)

type PostModerationTask struct {
	ID                 int64
	PostID             int64
	Provider           string
	Status             string
	ProviderTaskID     string
	SyncResultPayload  string
	AsyncResultPayload string
	FinalResultPayload string
	Conclusion         string
	CategoriesJSON     string
	Reason             string
	Attempts           int
	NextRetryAt        sql.NullTime
	LastError          string
	LastCheckedAt      sql.NullTime
	ReviewedAt         sql.NullTime
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PostModerationTaskUpdate struct {
	PostID             int64
	Status             string
	ProviderTaskID     string
	SyncResultPayload  string
	AsyncResultPayload string
	FinalResultPayload string
	Conclusion         string
	CategoriesJSON     string
	Reason             string
	Attempts           int
	NextRetryAt        *time.Time
	LastError          string
	LastCheckedAt      *time.Time
	ReviewedAt         *time.Time
}

type ContentViolationRecord struct {
	ID                  int64
	ViolationNo         string
	UserID              int64
	PostID              int64
	Source              string
	ActionType          string
	CategoriesJSON      string
	Reason              string
	EvidencePayloadJSON string
	MuteScope           string
	MuteDurationSeconds int64
	OperatorUserID      sql.NullInt64
	CreatedAt           time.Time
}

type UserRestriction struct {
	ID               int64
	RestrictionNo    string
	UserID           int64
	Scope            string
	Status           string
	Source           string
	Reason           string
	RelatedViolation sql.NullInt64
	OperatorUserID   sql.NullInt64
	StartsAt         time.Time
	EndsAt           sql.NullTime
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ViolationDecision struct {
	ViolationNo         string
	PostID              int64
	UserID              int64
	Source              string
	ActionType          string
	CategoriesJSON      string
	Reason              string
	EvidencePayloadJSON string
	MuteScope           string
	MuteDurationSeconds int64
	OperatorUserID      int64
}

type HumanReviewRecord struct {
	ID        int64
	PostID    int64
	Status    string
	Reason    string
	Source    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ModerationStore interface {
	GetOrCreateTask(ctx context.Context, postID int64, provider string) (PostModerationTask, error)
	UpdateTask(ctx context.Context, update PostModerationTaskUpdate) error
	ListTasksForRetry(ctx context.Context, now time.Time, limit int) ([]PostModerationTask, error)
	ActiveRestriction(ctx context.Context, userID int64, scope string) (UserRestriction, bool, error)
	ApplyViolation(ctx context.Context, decision ViolationDecision) (ContentViolationRecord, error)
	EnqueueHumanReview(ctx context.Context, postID int64, source, reason string) error
}

type moderationStore struct {
	db *gorm.DB
}

type postModerationTaskRecord struct {
	ID                 int64      `gorm:"column:id;primaryKey"`
	PostID             int64      `gorm:"column:post_id"`
	Provider           string     `gorm:"column:provider"`
	Status             string     `gorm:"column:status"`
	ProviderTaskID     string     `gorm:"column:provider_task_id"`
	SyncResultPayload  string     `gorm:"column:sync_result_payload"`
	AsyncResultPayload string     `gorm:"column:async_result_payload"`
	FinalResultPayload string     `gorm:"column:final_result_payload"`
	Conclusion         string     `gorm:"column:conclusion"`
	CategoriesJSON     string     `gorm:"column:categories"`
	Reason             string     `gorm:"column:reason"`
	Attempts           int        `gorm:"column:attempts"`
	NextRetryAt        *time.Time `gorm:"column:next_retry_at"`
	LastError          string     `gorm:"column:last_error"`
	LastCheckedAt      *time.Time `gorm:"column:last_checked_at"`
	ReviewedAt         *time.Time `gorm:"column:reviewed_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (postModerationTaskRecord) TableName() string {
	return "post_moderation_tasks"
}

type contentViolationRecordEntity struct {
	ID                  int64         `gorm:"column:id;primaryKey"`
	ViolationNo         string        `gorm:"column:violation_no"`
	UserID              int64         `gorm:"column:user_id"`
	PostID              int64         `gorm:"column:post_id"`
	Source              string        `gorm:"column:source"`
	ActionType          string        `gorm:"column:action_type"`
	CategoriesJSON      string        `gorm:"column:categories"`
	Reason              string        `gorm:"column:reason"`
	EvidencePayloadJSON string        `gorm:"column:evidence_payload"`
	MuteScope           string        `gorm:"column:mute_scope"`
	MuteDurationSeconds int64         `gorm:"column:mute_duration_seconds"`
	OperatorUserID      sql.NullInt64 `gorm:"column:operator_user_id"`
	CreatedAt           time.Time     `gorm:"column:created_at"`
}

func (contentViolationRecordEntity) TableName() string {
	return "content_violation_records"
}

type userRestrictionRecord struct {
	ID                 int64         `gorm:"column:id;primaryKey"`
	RestrictionNo      string        `gorm:"column:restriction_no"`
	UserID             int64         `gorm:"column:user_id"`
	Scope              string        `gorm:"column:scope"`
	Status             string        `gorm:"column:status"`
	Source             string        `gorm:"column:source"`
	Reason             string        `gorm:"column:reason"`
	RelatedViolationID sql.NullInt64 `gorm:"column:related_violation_id"`
	OperatorUserID     sql.NullInt64 `gorm:"column:operator_user_id"`
	StartsAt           time.Time     `gorm:"column:starts_at"`
	EndsAt             sql.NullTime  `gorm:"column:ends_at"`
	CreatedAt          time.Time     `gorm:"column:created_at"`
	UpdatedAt          time.Time     `gorm:"column:updated_at"`
}

func (userRestrictionRecord) TableName() string {
	return "user_restrictions"
}

type humanReviewRecordEntity struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	PostID    int64     `gorm:"column:post_id;uniqueIndex"`
	Status    string    `gorm:"column:status"`
	Reason    string    `gorm:"column:reason"`
	Source    string    `gorm:"column:source"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (humanReviewRecordEntity) TableName() string {
	return "human_review_queue"
}

type postRecord struct {
	ID        int64          `gorm:"column:id;primaryKey"`
	Status    int32          `gorm:"column:status"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
}

func (postRecord) TableName() string {
	return "posts"
}

func NewModerationStore(db *gorm.DB) ModerationStore {
	return &moderationStore{db: requireGormDB(db)}
}

func (s *moderationStore) ensureHumanReviewSchema(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&humanReviewRecordEntity{})
}

func (s *moderationStore) GetOrCreateTask(ctx context.Context, postID int64, provider string) (PostModerationTask, error) {
	now := time.Now()
	record := postModerationTaskRecord{
		PostID:    postID,
		Provider:  provider,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "post_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"provider":   provider,
			"updated_at": now,
		}),
	}).Create(&record).Error; err != nil {
		return PostModerationTask{}, err
	}

	var task postModerationTaskRecord
	if err := s.db.WithContext(ctx).Where("post_id = ?", postID).Take(&task).Error; err != nil {
		return PostModerationTask{}, err
	}
	return mapPostModerationTask(task), nil
}

func (s *moderationStore) UpdateTask(ctx context.Context, update PostModerationTaskUpdate) error {
	return s.db.WithContext(ctx).
		Model(&postModerationTaskRecord{}).
		Where("post_id = ?", update.PostID).
		Updates(map[string]any{
			"status":               update.Status,
			"provider_task_id":     update.ProviderTaskID,
			"sync_result_payload":  emptyJSONIfBlank(update.SyncResultPayload),
			"async_result_payload": emptyJSONIfBlank(update.AsyncResultPayload),
			"final_result_payload": emptyJSONIfBlank(update.FinalResultPayload),
			"conclusion":           update.Conclusion,
			"categories":           emptyJSONArrayIfBlank(update.CategoriesJSON),
			"reason":               update.Reason,
			"attempts":             update.Attempts,
			"next_retry_at":        update.NextRetryAt,
			"last_error":           update.LastError,
			"last_checked_at":      update.LastCheckedAt,
			"reviewed_at":          update.ReviewedAt,
			"updated_at":           gorm.Expr("NOW()"),
		}).Error
}

func (s *moderationStore) ListTasksForRetry(ctx context.Context, now time.Time, limit int) ([]PostModerationTask, error) {
	if limit <= 0 {
		limit = 20
	}
	var records []postModerationTaskRecord
	if err := s.db.WithContext(ctx).
		Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", []string{"waiting_async", "failed"}, now).
		Order("COALESCE(next_retry_at, created_at) ASC, id ASC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, err
	}
	tasks := make([]PostModerationTask, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, mapPostModerationTask(record))
	}
	return tasks, nil
}

func (s *moderationStore) ActiveRestriction(ctx context.Context, userID int64, scope string) (UserRestriction, bool, error) {
	var record userRestrictionRecord
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND scope = ? AND status = ? AND starts_at <= ?", userID, scope, RestrictionStatusActive, time.Now()).
		Where("ends_at IS NULL OR ends_at > ?", time.Now()).
		Order("ends_at DESC NULLS LAST, id DESC").
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UserRestriction{}, false, nil
	}
	if err != nil {
		return UserRestriction{}, false, err
	}
	return mapUserRestriction(record), true, nil
}

func (s *moderationStore) ApplyViolation(ctx context.Context, decision ViolationDecision) (ContentViolationRecord, error) {
	record := ContentViolationRecord{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&postRecord{}).
			Where("id = ? AND deleted_at IS NULL", decision.PostID).
			Updates(map[string]any{
				"status":     contentpb.PostStatus_POST_STATUS_BLOCKED,
				"updated_at": gorm.Expr("NOW()"),
			}).Error; err != nil {
			return err
		}

		entity := contentViolationRecordEntity{
			ViolationNo:         decision.ViolationNo,
			UserID:              decision.UserID,
			PostID:              decision.PostID,
			Source:              decision.Source,
			ActionType:          decision.ActionType,
			CategoriesJSON:      emptyJSONArrayIfBlank(decision.CategoriesJSON),
			Reason:              decision.Reason,
			EvidencePayloadJSON: emptyJSONIfBlank(decision.EvidencePayloadJSON),
			MuteScope:           decision.MuteScope,
			MuteDurationSeconds: decision.MuteDurationSeconds,
			OperatorUserID:      nullInt64(decision.OperatorUserID),
			CreatedAt:           time.Now(),
		}
		if err := tx.Create(&entity).Error; err != nil {
			return err
		}
		record = mapContentViolationRecord(entity)

		if decision.MuteDurationSeconds > 0 {
			if err := upsertActiveRestriction(tx, decision, record.ID); err != nil {
				return err
			}
		}

		return tx.Model(&postModerationTaskRecord{}).
			Where("post_id = ?", decision.PostID).
			Updates(map[string]any{
				"status":               "rejected",
				"conclusion":           "blocked",
				"categories":           emptyJSONArrayIfBlank(decision.CategoriesJSON),
				"reason":               decision.Reason,
				"final_result_payload": emptyJSONIfBlank(decision.EvidencePayloadJSON),
				"reviewed_at":          gorm.Expr("NOW()"),
				"last_checked_at":      gorm.Expr("NOW()"),
				"updated_at":           gorm.Expr("NOW()"),
			}).Error
	})
	return record, err
}

func (s *moderationStore) EnqueueHumanReview(ctx context.Context, postID int64, source, reason string) error {
	if err := s.ensureHumanReviewSchema(ctx); err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "post_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":     "pending",
			"reason":     reason,
			"source":     source,
			"updated_at": now,
		}),
	}).Create(&humanReviewRecordEntity{
		PostID:    postID,
		Status:    "pending",
		Reason:    reason,
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}

func upsertActiveRestriction(tx *gorm.DB, decision ViolationDecision, violationID int64) error {
	startsAt := time.Now()
	endsAt := startsAt.Add(time.Duration(decision.MuteDurationSeconds) * time.Second)

	var existing userRestrictionRecord
	err := tx.
		Where("user_id = ? AND scope = ? AND status = ?", decision.UserID, decision.MuteScope, RestrictionStatusActive).
		Where("ends_at IS NULL OR ends_at > ?", time.Now()).
		Order("ends_at DESC NULLS LAST, id DESC").
		Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		if existing.EndsAt.Valid && existing.EndsAt.Time.After(endsAt) {
			endsAt = existing.EndsAt.Time
		}
		return tx.Model(&userRestrictionRecord{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{
				"source":               decision.Source,
				"reason":               decision.Reason,
				"related_violation_id": violationID,
				"operator_user_id":     nullIfZero(decision.OperatorUserID),
				"ends_at":              endsAt,
				"updated_at":           gorm.Expr("NOW()"),
			}).Error
	}

	return tx.Create(&userRestrictionRecord{
		RestrictionNo:      decision.ViolationNo + "_restriction",
		UserID:             decision.UserID,
		Scope:              decision.MuteScope,
		Status:             RestrictionStatusActive,
		Source:             decision.Source,
		Reason:             decision.Reason,
		RelatedViolationID: sql.NullInt64{Int64: violationID, Valid: true},
		OperatorUserID:     nullInt64(decision.OperatorUserID),
		StartsAt:           startsAt,
		EndsAt:             sql.NullTime{Time: endsAt, Valid: true},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}).Error
}

func emptyJSONIfBlank(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}

func emptyJSONArrayIfBlank(value string) string {
	if value == "" {
		return "[]"
	}
	return value
}

func nullInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func nullIfZero(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func mapPostModerationTask(record postModerationTaskRecord) PostModerationTask {
	return PostModerationTask{
		ID:                 record.ID,
		PostID:             record.PostID,
		Provider:           record.Provider,
		Status:             record.Status,
		ProviderTaskID:     record.ProviderTaskID,
		SyncResultPayload:  record.SyncResultPayload,
		AsyncResultPayload: record.AsyncResultPayload,
		FinalResultPayload: record.FinalResultPayload,
		Conclusion:         record.Conclusion,
		CategoriesJSON:     record.CategoriesJSON,
		Reason:             record.Reason,
		Attempts:           record.Attempts,
		NextRetryAt:        toNullTime(record.NextRetryAt),
		LastError:          record.LastError,
		LastCheckedAt:      toNullTime(record.LastCheckedAt),
		ReviewedAt:         toNullTime(record.ReviewedAt),
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func mapUserRestriction(record userRestrictionRecord) UserRestriction {
	return UserRestriction{
		ID:               record.ID,
		RestrictionNo:    record.RestrictionNo,
		UserID:           record.UserID,
		Scope:            record.Scope,
		Status:           record.Status,
		Source:           record.Source,
		Reason:           record.Reason,
		RelatedViolation: record.RelatedViolationID,
		OperatorUserID:   record.OperatorUserID,
		StartsAt:         record.StartsAt,
		EndsAt:           record.EndsAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func mapContentViolationRecord(record contentViolationRecordEntity) ContentViolationRecord {
	return ContentViolationRecord{
		ID:                  record.ID,
		ViolationNo:         record.ViolationNo,
		UserID:              record.UserID,
		PostID:              record.PostID,
		Source:              record.Source,
		ActionType:          record.ActionType,
		CategoriesJSON:      record.CategoriesJSON,
		Reason:              record.Reason,
		EvidencePayloadJSON: record.EvidencePayloadJSON,
		MuteScope:           record.MuteScope,
		MuteDurationSeconds: record.MuteDurationSeconds,
		OperatorUserID:      record.OperatorUserID,
		CreatedAt:           record.CreatedAt,
	}
}

func toNullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}
