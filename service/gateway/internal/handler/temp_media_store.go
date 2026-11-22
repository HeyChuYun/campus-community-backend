package handler

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	tempMediaStatusIssued  = "issued"
	tempMediaStatusPending = "pending"
	tempMediaStatusUsed    = "used"
	tempMediaStatusExpired = "expired"
)

var tempMediaSchemaOnce sync.Once

type tempMediaRecord struct {
	MediaID      string        `gorm:"column:media_id;primaryKey;type:varchar(64)"`
	UserID       int64         `gorm:"column:user_id;not null;index:idx_temp_media_user_status_expires,priority:1"`
	ObjectKey    string        `gorm:"column:object_key;type:text;not null;uniqueIndex"`
	Status       string        `gorm:"column:status;type:varchar(16);not null;index:idx_temp_media_user_status_expires,priority:2"`
	ExpiresAt    time.Time     `gorm:"column:expires_at;not null;index:idx_temp_media_user_status_expires,priority:3,sort:desc"`
	PostID       sql.NullInt64 `gorm:"column:post_id"`
	MediaType    int32         `gorm:"column:media_type;not null;default:0"`
	ContentType  string        `gorm:"column:content_type;type:varchar(128);not null;default:''"`
	OriginalName string        `gorm:"column:original_name;type:varchar(255);not null;default:''"`
	SizeBytes    int64         `gorm:"column:size_bytes;not null;default:0"`
	CreatedAt    time.Time     `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time     `gorm:"column:updated_at;autoUpdateTime"`
}

func (tempMediaRecord) TableName() string {
	return "temp_media"
}

func ensureTempMediaSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("gorm db is nil")
	}
	var err error
	tempMediaSchemaOnce.Do(func() {
		err = db.WithContext(ctx).AutoMigrate(&tempMediaRecord{})
	})
	return err
}

func upsertTempMedia(ctx context.Context, db *gorm.DB, record tempMediaRecord) error {
	if err := ensureTempMediaSchema(ctx, db); err != nil {
		return err
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "media_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"user_id":       record.UserID,
			"object_key":    record.ObjectKey,
			"status":        record.Status,
			"expires_at":    record.ExpiresAt,
			"post_id":       nullInt64ToAny(record.PostID),
			"media_type":    record.MediaType,
			"content_type":  record.ContentType,
			"original_name": record.OriginalName,
			"size_bytes":    record.SizeBytes,
			"updated_at":    gorm.Expr("NOW()"),
		}),
	}).Create(&record).Error
}

func listTempMediaByIDs(ctx context.Context, db *gorm.DB, mediaIDs []string) ([]tempMediaRecord, error) {
	if err := ensureTempMediaSchema(ctx, db); err != nil {
		return nil, err
	}
	cleaned := make([]string, 0, len(mediaIDs))
	seen := make(map[string]struct{}, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		trimmed := strings.TrimSpace(mediaID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return []tempMediaRecord{}, nil
	}

	items := make([]tempMediaRecord, 0, len(cleaned))
	if err := db.WithContext(ctx).
		Where("media_id IN ?", cleaned).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func markTempMediaUsed(ctx context.Context, tx *gorm.DB, mediaIDs []string, postID int64) error {
	return tx.WithContext(ctx).
		Model(&tempMediaRecord{}).
		Where("media_id IN ?", mediaIDs).
		Updates(map[string]any{
			"status":     tempMediaStatusUsed,
			"post_id":    postID,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

func nullInt64ToAny(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}
