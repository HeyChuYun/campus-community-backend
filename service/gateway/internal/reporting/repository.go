package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	reportDefaultDailyTargetLimit    = 3
	reportDefaultSameTargetCooldown  = 7
	reportDefaultMinReasonLength     = 15
	reportDefaultMaxReasonLength     = 500
	reportDefaultMaxEvidenceCount    = 6
	reportDefaultPageSize            = 20
	reportDefaultMaxEvidenceSizeByte = 8 * 1024 * 1024
	reportDefaultPage                = 1
	reportActionCreateNote           = "user submitted report"
	reportActionAppealCreateNote     = "user submitted post appeal"
	reportEvidenceMediaTypeImage     = "image"
)

type Repository interface {
	Settings(ctx context.Context) (Settings, error)
	TargetUserSnapshot(ctx context.Context, userID int64) (map[string]any, error)
	LatestReporterTargetStatus(ctx context.Context, reporterUserID, targetUserID int64) (string, time.Time, bool, error)
	LatestReporterPostStatus(ctx context.Context, reporterUserID, postID int64) (string, time.Time, bool, error)
	TodayDistinctTargetCount(ctx context.Context, reporterUserID int64) (int, error)
	ReportedTargetToday(ctx context.Context, reporterUserID, targetUserID int64) (bool, error)
	CreateUserReport(ctx context.Context, input CreateUserReportInput, reportNo string, snapshotRaw []byte) (CreateUserReportResult, error)
	PostAppealContext(ctx context.Context, requesterUserID, postID int64) (PostAppealContext, error)
	CreatePostAppeal(ctx context.Context, input CreatePostAppealInput, reportNo string, snapshotRaw []byte) (CreatePostAppealResult, error)
	AdminList(ctx context.Context, filter AdminListFilter) (AdminListResult, error)
	AdminDetail(ctx context.Context, reportID int64) (AdminReportDetail, bool, error)
	UpdateAdminStatus(ctx context.Context, input AdminStatusUpdate) error
}

type GormRepository struct {
	db *gorm.DB
}

type reportSettingRow struct {
	SettingKey   string `gorm:"column:setting_key"`
	SettingValue string `gorm:"column:setting_value"`
}

func (reportSettingRow) TableName() string {
	return "report_settings"
}

type reportUserRow struct {
	ID            int64          `gorm:"column:id"`
	Account       string         `gorm:"column:account"`
	Nickname      string         `gorm:"column:nickname"`
	AvatarURL     string         `gorm:"column:avatar_url"`
	AccountStatus int            `gorm:"column:account_status"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (reportUserRow) TableName() string {
	return "users"
}

type reportRow struct {
	ID                 int64      `gorm:"column:id;primaryKey"`
	ReportNo           string     `gorm:"column:report_no"`
	ReporterUserID     *int64     `gorm:"column:reporter_user_id"`
	TargetType         string     `gorm:"column:target_type"`
	TargetUserID       *int64     `gorm:"column:target_user_id"`
	TargetPostID       *int64     `gorm:"column:target_post_id"`
	CategoryCode       string     `gorm:"column:category_code"`
	ReasonText         string     `gorm:"column:reason_text"`
	Status             string     `gorm:"column:status"`
	TargetSnapshotJSON []byte     `gorm:"column:target_snapshot_json"`
	LatestDecisionCode string     `gorm:"column:latest_decision_code"`
	LatestDecisionNote string     `gorm:"column:latest_decision_note"`
	AssignedToUserID   *int64     `gorm:"column:assigned_to_user_id"`
	ClosedAt           *time.Time `gorm:"column:closed_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (reportRow) TableName() string {
	return "reports"
}

type reportEvidenceRow struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	ReportID  int64     `gorm:"column:report_id"`
	MediaType string    `gorm:"column:media_type"`
	FileURL   string    `gorm:"column:file_url"`
	FileSize  int64     `gorm:"column:file_size"`
	Width     int       `gorm:"column:width"`
	Height    int       `gorm:"column:height"`
	SortOrder int       `gorm:"column:sort_order"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (reportEvidenceRow) TableName() string {
	return "report_evidences"
}

type reportActionRow struct {
	ID             int64     `gorm:"column:id;primaryKey"`
	ReportID       int64     `gorm:"column:report_id"`
	OperatorUserID int64     `gorm:"column:operator_user_id"`
	ActionType     string    `gorm:"column:action_type"`
	FromStatus     string    `gorm:"column:from_status"`
	ToStatus       string    `gorm:"column:to_status"`
	Note           string    `gorm:"column:note"`
	ExtraJSON      []byte    `gorm:"column:extra_json"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (reportActionRow) TableName() string {
	return "report_actions"
}

type postAppealPostRow struct {
	ID           int64          `gorm:"column:id"`
	AuthorUserID int64          `gorm:"column:author_user_id"`
	ContentText  string         `gorm:"column:content_text"`
	Status       int32          `gorm:"column:status"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (postAppealPostRow) TableName() string {
	return "posts"
}

type violationRecordRow struct {
	PostID              int64     `gorm:"column:post_id"`
	ViolationNo         string    `gorm:"column:violation_no"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	Source              string    `gorm:"column:source"`
	Reason              string    `gorm:"column:reason"`
	Categories          []byte    `gorm:"column:categories"`
	MuteScope           string    `gorm:"column:mute_scope"`
	MuteDurationSeconds int64     `gorm:"column:mute_duration_seconds"`
}

func (violationRecordRow) TableName() string {
	return "content_violation_records"
}

type userRestrictionRow struct {
	ID     int64      `gorm:"column:id"`
	UserID int64      `gorm:"column:user_id"`
	Scope  string     `gorm:"column:scope"`
	Status string     `gorm:"column:status"`
	EndsAt *time.Time `gorm:"column:ends_at"`
}

func (userRestrictionRow) TableName() string {
	return "user_restrictions"
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: requireGormDB(db)}
}

func (r *GormRepository) Settings(ctx context.Context) (Settings, error) {
	settings := Settings{
		DailyTargetLimit:    reportDefaultDailyTargetLimit,
		SameTargetCooldown:  reportDefaultSameTargetCooldown,
		MinReasonLength:     reportDefaultMinReasonLength,
		MaxReasonLength:     reportDefaultMaxReasonLength,
		MaxEvidenceCount:    reportDefaultMaxEvidenceCount,
		MaxEvidenceSizeByte: reportDefaultMaxEvidenceSizeByte,
	}
	var rows []reportSettingRow
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return settings, err
	}
	for _, row := range rows {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(row.SettingValue))
		if parseErr != nil {
			continue
		}
		switch strings.TrimSpace(row.SettingKey) {
		case "daily_report_target_limit":
			settings.DailyTargetLimit = parsed
		case "same_target_cooldown_days":
			settings.SameTargetCooldown = parsed
		case "min_reason_length":
			settings.MinReasonLength = parsed
		case "max_reason_length":
			settings.MaxReasonLength = parsed
		case "max_evidence_count":
			settings.MaxEvidenceCount = parsed
		case "max_evidence_size_bytes":
			settings.MaxEvidenceSizeByte = int64(parsed)
		}
	}
	return settings, nil
}

func (r *GormRepository) TargetUserSnapshot(ctx context.Context, userID int64) (map[string]any, error) {
	var user reportUserRow
	err := r.db.WithContext(ctx).
		Select("id", "account", "nickname", "avatar_url", "account_status").
		First(&user, "id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"user_id":        userID,
		"account":        user.Account,
		"nickname":       user.Nickname,
		"avatar_url":     user.AvatarURL,
		"account_status": user.AccountStatus,
	}, nil
}

func (r *GormRepository) LatestReporterTargetStatus(ctx context.Context, reporterUserID, targetUserID int64) (string, time.Time, bool, error) {
	var report reportRow
	err := r.db.WithContext(ctx).
		Select("status", "created_at").
		Where("reporter_user_id = ? AND target_type = ? AND target_user_id = ?", reporterUserID, TargetTypeUser, targetUserID).
		Order("created_at DESC, id DESC").
		First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}
	return report.Status, report.CreatedAt, true, nil
}

func (r *GormRepository) LatestReporterPostStatus(ctx context.Context, reporterUserID, postID int64) (string, time.Time, bool, error) {
	var report reportRow
	err := r.db.WithContext(ctx).
		Select("status", "created_at").
		Where("reporter_user_id = ? AND target_type = ? AND target_post_id = ?", reporterUserID, TargetTypePost, postID).
		Order("created_at DESC, id DESC").
		First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}
	return report.Status, report.CreatedAt, true, nil
}

func (r *GormRepository) TodayDistinctTargetCount(ctx context.Context, reporterUserID int64) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&reportRow{}).
		Distinct("target_user_id").
		Where("reporter_user_id = ? AND target_type = ? AND created_at >= ?", reporterUserID, TargetTypeUser, startOfToday()).
		Count(&count).Error
	return int(count), err
}

func (r *GormRepository) ReportedTargetToday(ctx context.Context, reporterUserID, targetUserID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&reportRow{}).
		Where("reporter_user_id = ? AND target_type = ? AND target_user_id = ? AND created_at >= ?", reporterUserID, TargetTypeUser, targetUserID, startOfToday()).
		Count(&count).Error
	return count > 0, err
}

func (r *GormRepository) CreateUserReport(ctx context.Context, input CreateUserReportInput, reportNo string, snapshotRaw []byte) (CreateUserReportResult, error) {
	var reportID int64
	now := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reporterUserID := input.ReporterUserID
		targetUserID := input.TargetUserID
		report := reportRow{
			ReportNo:           reportNo,
			ReporterUserID:     &reporterUserID,
			TargetType:         TargetTypeUser,
			TargetUserID:       &targetUserID,
			CategoryCode:       input.CategoryCode,
			ReasonText:         input.ReasonText,
			Status:             StatusPending,
			TargetSnapshotJSON: snapshotRaw,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := tx.Create(&report).Error; err != nil {
			return err
		}
		reportID = report.ID

		evidences := make([]reportEvidenceRow, 0, len(input.Evidences))
		for index, item := range input.Evidences {
			evidences = append(evidences, reportEvidenceRow{
				ReportID:  reportID,
				MediaType: reportEvidenceMediaTypeImage,
				FileURL:   strings.TrimSpace(item.FileURL),
				FileSize:  item.FileSize,
				Width:     item.Width,
				Height:    item.Height,
				SortOrder: index,
				CreatedAt: now,
			})
		}
		if len(evidences) > 0 {
			if err := tx.Create(&evidences).Error; err != nil {
				return err
			}
		}

		return tx.Create(&reportActionRow{
			ReportID:       reportID,
			OperatorUserID: input.ReporterUserID,
			ActionType:     "create",
			FromStatus:     "",
			ToStatus:       StatusPending,
			Note:           reportActionCreateNote,
			ExtraJSON:      []byte("{}"),
			CreatedAt:      now,
		}).Error
	})
	if err != nil {
		return CreateUserReportResult{}, err
	}
	return CreateUserReportResult{ReportID: reportID, ReportNo: reportNo, Status: StatusPending}, nil
}

func (r *GormRepository) PostAppealContext(ctx context.Context, requesterUserID, postID int64) (PostAppealContext, error) {
	var post postAppealPostRow
	err := r.db.WithContext(ctx).
		Select("id", "author_user_id", "content_text", "status").
		First(&post, "id = ? AND author_user_id = ?", postID, requesterUserID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PostAppealContext{}, ErrNotFound
	}
	if err != nil {
		return PostAppealContext{}, err
	}

	result := PostAppealContext{
		PostID:       post.ID,
		AuthorUserID: post.AuthorUserID,
		ContentText:  post.ContentText,
		PostStatus:   post.Status,
	}

	var violation violationRecordRow
	err = r.db.WithContext(ctx).
		Select("violation_no", "created_at", "source", "reason", "categories", "mute_scope", "mute_duration_seconds").
		Where("post_id = ?", postID).
		Order("created_at DESC, id DESC").
		First(&violation).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return PostAppealContext{}, err
	}
	if err == nil {
		result.LatestViolationNo = violation.ViolationNo
		result.LatestViolationAt = &violation.CreatedAt
		result.LatestViolationSource = violation.Source
		result.LatestReason = violation.Reason
		result.LatestCategories = parseCategoryArray(string(violation.Categories))
		result.MuteScope = violation.MuteScope
		result.MuteDurationSeconds = violation.MuteDurationSeconds
	}

	var restriction userRestrictionRow
	err = r.db.WithContext(ctx).
		Select("ends_at").
		Where("user_id = ? AND scope = ? AND status = ?", post.AuthorUserID, "post_publish", "active").
		Where("ends_at IS NULL OR ends_at > ?", time.Now()).
		Order("ends_at DESC NULLS LAST, id DESC").
		First(&restriction).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return PostAppealContext{}, err
	}
	if err == nil {
		result.PostMuteUntil = restriction.EndsAt
	}
	return result, nil
}

func (r *GormRepository) CreatePostAppeal(ctx context.Context, input CreatePostAppealInput, reportNo string, snapshotRaw []byte) (CreatePostAppealResult, error) {
	var reportID int64
	now := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reporterUserID := input.ReporterUserID
		targetUserID := input.TargetUserID
		targetPostID := input.TargetPostID
		report := reportRow{
			ReportNo:           reportNo,
			ReporterUserID:     &reporterUserID,
			TargetType:         TargetTypePost,
			TargetUserID:       &targetUserID,
			TargetPostID:       &targetPostID,
			CategoryCode:       "post_appeal",
			ReasonText:         input.ReasonText,
			Status:             StatusPending,
			TargetSnapshotJSON: snapshotRaw,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := tx.Create(&report).Error; err != nil {
			return err
		}
		reportID = report.ID

		evidences := make([]reportEvidenceRow, 0, len(input.Evidences))
		for index, item := range input.Evidences {
			evidences = append(evidences, reportEvidenceRow{
				ReportID:  reportID,
				MediaType: reportEvidenceMediaTypeImage,
				FileURL:   strings.TrimSpace(item.FileURL),
				FileSize:  item.FileSize,
				Width:     item.Width,
				Height:    item.Height,
				SortOrder: index,
				CreatedAt: now,
			})
		}
		if len(evidences) > 0 {
			if err := tx.Create(&evidences).Error; err != nil {
				return err
			}
		}

		return tx.Create(&reportActionRow{
			ReportID:       reportID,
			OperatorUserID: input.ReporterUserID,
			ActionType:     "create_post_appeal",
			FromStatus:     "",
			ToStatus:       StatusPending,
			Note:           reportActionAppealCreateNote,
			ExtraJSON:      []byte("{}"),
			CreatedAt:      now,
		}).Error
	})
	if err != nil {
		return CreatePostAppealResult{}, err
	}
	return CreatePostAppealResult{ReportID: reportID, ReportNo: reportNo, Status: StatusPending}, nil
}

func (r *GormRepository) AdminList(ctx context.Context, filter AdminListFilter) (AdminListResult, error) {
	query := r.adminListQuery(ctx, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AdminListResult{}, err
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = reportDefaultPageSize
	}
	page := filter.Page
	if page <= 0 {
		page = reportDefaultPage
	}
	offset := (page - 1) * pageSize

	var rows []reportRow
	if err := query.
		Select("id", "report_no", "reporter_user_id", "target_type", "target_user_id", "target_post_id", "category_code", "reason_text", "status", "latest_decision_code", "latest_decision_note", "assigned_to_user_id", "created_at", "updated_at").
		Order("created_at DESC, id DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return AdminListResult{}, err
	}

	items := make([]AdminReportItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, AdminReportItem{
			ReportID:           row.ID,
			ReportNo:           row.ReportNo,
			ReporterUserID:     derefInt64(row.ReporterUserID),
			TargetType:         row.TargetType,
			TargetUserID:       derefInt64(row.TargetUserID),
			TargetPostID:       derefInt64(row.TargetPostID),
			CategoryCode:       row.CategoryCode,
			ReasonText:         row.ReasonText,
			Status:             row.Status,
			LatestDecisionCode: row.LatestDecisionCode,
			LatestDecisionNote: row.LatestDecisionNote,
			AssignedToUserID:   derefInt64(row.AssignedToUserID),
			CreatedAt:          row.CreatedAt,
			UpdatedAt:          row.UpdatedAt,
		})
	}
	return AdminListResult{Items: items, Total: total}, nil
}

func (r *GormRepository) AdminDetail(ctx context.Context, reportID int64) (AdminReportDetail, bool, error) {
	var row reportRow
	err := r.db.WithContext(ctx).
		Select("id", "report_no", "reporter_user_id", "target_type", "target_user_id", "target_post_id", "category_code", "reason_text", "status", "target_snapshot_json", "latest_decision_code", "latest_decision_note", "assigned_to_user_id", "closed_at", "created_at", "updated_at").
		First(&row, "id = ?", reportID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AdminReportDetail{}, false, nil
	}
	if err != nil {
		return AdminReportDetail{}, false, err
	}

	report := AdminReportDetailItem{
		ReportID:           row.ID,
		ReportNo:           row.ReportNo,
		ReporterUserID:     derefInt64(row.ReporterUserID),
		TargetType:         row.TargetType,
		TargetUserID:       derefInt64(row.TargetUserID),
		TargetPostID:       derefInt64(row.TargetPostID),
		CategoryCode:       row.CategoryCode,
		ReasonText:         row.ReasonText,
		Status:             row.Status,
		TargetSnapshot:     json.RawMessage(row.TargetSnapshotJSON),
		LatestDecisionCode: row.LatestDecisionCode,
		LatestDecisionNote: row.LatestDecisionNote,
		AssignedToUserID:   derefInt64(row.AssignedToUserID),
		ClosedAt:           row.ClosedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}

	evidences, err := r.adminEvidences(ctx, reportID)
	if err != nil {
		return AdminReportDetail{}, false, err
	}
	actions, err := r.adminActions(ctx, reportID)
	if err != nil {
		return AdminReportDetail{}, false, err
	}
	return AdminReportDetail{Report: report, Evidences: evidences, Actions: actions}, true, nil
}

func (r *GormRepository) UpdateAdminStatus(ctx context.Context, input AdminStatusUpdate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current reportRow
		if err := tx.Select("status", "assigned_to_user_id").First(&current, "id = ?", input.ReportID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		assignedTo := input.AssignedToUID
		if assignedTo <= 0 {
			assignedTo = derefInt64(current.AssignedToUserID)
		}

		updates := map[string]any{
			"status":               input.Status,
			"latest_decision_code": input.DecisionCode,
			"latest_decision_note": input.DecisionNote,
			"assigned_to_user_id":  nullIfZero(assignedTo),
			"updated_at":           gorm.Expr("NOW()"),
			"closed_at":            nil,
		}
		if strings.HasPrefix(input.Status, "resolved_") || input.Status == StatusClosed {
			updates["closed_at"] = gorm.Expr("NOW()")
		}

		if err := tx.Model(&reportRow{}).Where("id = ?", input.ReportID).Updates(updates).Error; err != nil {
			return err
		}

		extraRaw, err := json.Marshal(map[string]any{
			"decision_code":       input.DecisionCode,
			"assigned_to_user_id": assignedTo,
		})
		if err != nil {
			return fmt.Errorf("marshal report action extra: %w", err)
		}
		return tx.Create(&reportActionRow{
			ReportID:       input.ReportID,
			OperatorUserID: input.OperatorUserID,
			ActionType:     "admin_update_status",
			FromStatus:     current.Status,
			ToStatus:       input.Status,
			Note:           input.DecisionNote,
			ExtraJSON:      extraRaw,
			CreatedAt:      time.Now(),
		}).Error
	})
}

func (r *GormRepository) adminEvidences(ctx context.Context, reportID int64) ([]EvidenceItem, error) {
	var rows []reportEvidenceRow
	if err := r.db.WithContext(ctx).
		Where("report_id = ?", reportID).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]EvidenceItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, EvidenceItem{
			ID:        row.ID,
			MediaType: row.MediaType,
			FileURL:   row.FileURL,
			FileSize:  row.FileSize,
			Width:     row.Width,
			Height:    row.Height,
			SortOrder: row.SortOrder,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

func (r *GormRepository) adminActions(ctx context.Context, reportID int64) ([]ActionItem, error) {
	var rows []reportActionRow
	if err := r.db.WithContext(ctx).
		Where("report_id = ?", reportID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]ActionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ActionItem{
			ID:             row.ID,
			OperatorUserID: row.OperatorUserID,
			ActionType:     row.ActionType,
			FromStatus:     row.FromStatus,
			ToStatus:       row.ToStatus,
			Note:           row.Note,
			Extra:          json.RawMessage(row.ExtraJSON),
			CreatedAt:      row.CreatedAt,
		})
	}
	return items, nil
}

func (r *GormRepository) adminListQuery(ctx context.Context, filter AdminListFilter) *gorm.DB {
	query := r.db.WithContext(ctx).Model(&reportRow{})
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.TargetType) != "" {
		query = query.Where("target_type = ?", strings.TrimSpace(filter.TargetType))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where(
			"report_no ILIKE ? OR reason_text ILIKE ? OR category_code ILIKE ? OR CAST(COALESCE(target_user_id, 0) AS text) ILIKE ? OR CAST(COALESCE(target_post_id, 0) AS text) ILIKE ?",
			keyword, keyword, keyword, keyword, keyword,
		)
	}
	return query
}

func parseCategoryArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func nullIfZero(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func startOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("reporting: gorm db is required")
	}
	return db
}
