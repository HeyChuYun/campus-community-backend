package appversion

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db *gorm.DB
}

type Record struct {
	ID                  int64     `gorm:"column:id"`
	VersionName         string    `gorm:"column:version_name"`
	VersionCode         int64     `gorm:"column:version_code"`
	Platform            string    `gorm:"column:platform"`
	DownloadURL         string    `gorm:"column:download_url"`
	AppStoreURL         string    `gorm:"column:app_store_url"`
	ForceUpdate         bool      `gorm:"column:force_update"`
	MinSupportedVersion string    `gorm:"column:min_supported_version"`
	UpdateLog           string    `gorm:"column:update_log"`
	IsActive            bool      `gorm:"column:is_active"`
	CreatedAt           time.Time `gorm:"column:created_at"`
}

type UpsertInput struct {
	VersionName         string
	VersionCode         int64
	Platform            string
	DownloadURL         string
	AppStoreURL         string
	ForceUpdate         bool
	MinSupportedVersion string
	UpdateLog           string
	IsActive            bool
	CreatedBy           string
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: requireGormDB(db)}
}

func (s *Service) Latest(ctx context.Context, platform string) (Record, bool, error) {
	var items []Record
	err := s.db.WithContext(ctx).
		Table("app_versions").
		Where("platform = ? AND is_active = ?", NormalizePlatform(platform), true).
		Order("created_at DESC, id DESC").
		Limit(100).
		Find(&items).Error
	if err != nil {
		return Record{}, false, err
	}
	if len(items) == 0 {
		return Record{}, false, nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if CompareRecord(item, latest) > 0 {
			latest = item
		}
	}
	return latest, true, nil
}

func (s *Service) List(ctx context.Context, platform string, limit int) ([]Record, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var items []Record
	err := s.db.WithContext(ctx).
		Table("app_versions").
		Where("platform = ?", NormalizePlatform(platform)).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (s *Service) Upsert(ctx context.Context, input UpsertInput) (Record, error) {
	now := time.Now()
	record := map[string]any{
		"version_name":          strings.TrimSpace(input.VersionName),
		"version_code":          input.VersionCode,
		"platform":              NormalizePlatform(input.Platform),
		"download_url":          strings.TrimSpace(input.DownloadURL),
		"app_store_url":         strings.TrimSpace(input.AppStoreURL),
		"force_update":          input.ForceUpdate,
		"min_supported_version": strings.TrimSpace(input.MinSupportedVersion),
		"update_log":            strings.TrimSpace(input.UpdateLog),
		"is_active":             input.IsActive,
		"created_by":            strings.TrimSpace(input.CreatedBy),
		"created_at":            now,
		"updated_at":            now,
	}
	err := s.db.WithContext(ctx).Table("app_versions").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "platform"}, {Name: "version_name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"version_code":          input.VersionCode,
			"download_url":          strings.TrimSpace(input.DownloadURL),
			"app_store_url":         strings.TrimSpace(input.AppStoreURL),
			"force_update":          input.ForceUpdate,
			"min_supported_version": strings.TrimSpace(input.MinSupportedVersion),
			"update_log":            strings.TrimSpace(input.UpdateLog),
			"is_active":             input.IsActive,
			"updated_at":            gorm.Expr("NOW()"),
		}),
	}).Create(record).Error
	if err != nil {
		return Record{}, err
	}

	var out Record
	err = s.db.WithContext(ctx).
		Table("app_versions").
		Where("platform = ? AND version_name = ?", record["platform"], record["version_name"]).
		First(&out).Error
	return out, err
}

func NormalizePlatform(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "ios" {
		return "ios"
	}
	return "android"
}

func CompareRecord(a, b Record) int {
	if a.VersionCode != b.VersionCode {
		if a.VersionCode > b.VersionCode {
			return 1
		}
		return -1
	}
	return SemverCompare(a.VersionName, b.VersionName)
}

func SemverCompare(left, right string) int {
	left = strings.TrimSpace(strings.TrimPrefix(left, "v"))
	right = strings.TrimSpace(strings.TrimPrefix(right, "v"))
	if left == "" && right == "" {
		return 0
	}
	if left == "" {
		return -1
	}
	if right == "" {
		return 1
	}
	lp := parseVersionParts(left)
	rp := parseVersionParts(right)
	max := len(lp)
	if len(rp) > max {
		max = len(rp)
	}
	for i := 0; i < max; i++ {
		lv, rv := int64(0), int64(0)
		if i < len(lp) {
			lv = lp[i]
		}
		if i < len(rp) {
			rv = rp[i]
		}
		if lv > rv {
			return 1
		}
		if lv < rv {
			return -1
		}
	}
	return 0
}

func parseVersionParts(value string) []int64 {
	chunks := strings.FieldsFunc(value, func(r rune) bool {
		return r == '.' || r == '-' || r == '+'
	})
	parts := make([]int64, 0, len(chunks))
	for _, chunk := range chunks {
		var n int64
		for _, r := range strings.TrimSpace(chunk) {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int64(r-'0')
		}
		parts = append(parts, n)
	}
	return parts
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("update appversion: gorm db is required")
	}
	return db
}
