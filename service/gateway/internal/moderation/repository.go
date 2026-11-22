package moderation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"hfjj/backend/proto/contentpb"

	"gorm.io/gorm"
)

const (
	moderationFilterAllStatus = 0
)

type Repository interface {
	ListPosts(ctx context.Context, filter PostListFilter) (PostListResult, error)
	ListUserViolations(ctx context.Context, filter UserViolationFilter) (UserViolationResult, error)
	RestorePostVisibility(ctx context.Context, postID int64) (bool, error)
}

type PostListFilter struct {
	Page       int
	PageSize   int
	Query      string
	Status     int
	CampusCode string
}

type PostListResult struct {
	Items []PostListItem
	Total int64
}

type PostListItem struct {
	PostID            int64
	AuthorUserID      int64
	AuthorAccount     string
	AuthorNickname    string
	ContentText       string
	Status            int32
	CampusCode        string
	Visibility        int32
	LikeCount         int64
	CommentCount      int64
	FavoriteCount     int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	LatestViolationAt *time.Time
	LatestReason      string
	LatestSource      string
	LatestCategories  []string
	PostMuteUntil     *time.Time
}

type UserViolationFilter struct {
	UserID   int64
	Page     int
	PageSize int
}

type UserViolationResult struct {
	Items []UserViolationItem
	Total int64
}

type UserViolationItem struct {
	ViolationNo         string
	PostID              int64
	Source              string
	Categories          []string
	Reason              string
	MuteScope           string
	MuteDurationSeconds int64
	OperatorUserID      int64
	CreatedAt           time.Time
}

type ViolationCategoryOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type gormRepository struct {
	db *gorm.DB
}

type postRow struct {
	PostID            int64
	AuthorUserID      int64
	AuthorAccount     string
	AuthorNickname    string
	ContentText       string
	Status            int32
	CampusCode        string
	Visibility        int32
	LikeCount         int64
	CommentCount      int64
	FavoriteCount     int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	LatestViolationAt *time.Time
	LatestReason      string
	LatestSource      string
	LatestCategories  string
	PostMuteUntil     *time.Time
}

type violationRow struct {
	ViolationNo         string
	PostID              int64
	Source              string
	Categories          string
	Reason              string
	MuteScope           string
	MuteDurationSeconds int64
	OperatorUserID      int64
	CreatedAt           time.Time
}

func NewGormRepository(db *gorm.DB) Repository {
	return &gormRepository{db: requireDB(db)}
}

func (r *gormRepository) ListPosts(ctx context.Context, filter PostListFilter) (PostListResult, error) {
	normalizePostListFilter(&filter)

	base := r.db.WithContext(ctx).Table("posts p").
		Joins("join users u on u.id = p.author_user_id").
		Where("p.deleted_at is null")
	if filter.Status > moderationFilterAllStatus {
		base = base.Where("p.status = ?", filter.Status)
	}
	if campus := normalizeCampusCode(filter.CampusCode); campus != "" && campus != "all" {
		base = base.Where("p.campus_code in (?, ?)", "all", campus)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where(
			"(u.account ilike ? or u.nickname ilike ? or p.content_text ilike ? or cast(p.id as text) ilike ? or cast(p.author_user_id as text) ilike ?)",
			like,
			like,
			like,
			like,
			like,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return PostListResult{}, err
	}

	var rows []postRow
	if err := base.
		Select(`
p.id as post_id,
p.author_user_id,
u.account as author_account,
u.nickname as author_nickname,
p.content_text,
p.status,
p.campus_code,
p.visibility,
p.like_count,
p.comment_count,
p.favorite_count,
p.created_at,
p.updated_at,
(
    select vr.created_at
    from content_violation_records vr
    where vr.post_id = p.id
    order by vr.created_at desc, vr.id desc
    limit 1
) as latest_violation_at,
coalesce((
    select vr.reason
    from content_violation_records vr
    where vr.post_id = p.id
    order by vr.created_at desc, vr.id desc
    limit 1
), '') as latest_reason,
coalesce((
    select vr.source
    from content_violation_records vr
    where vr.post_id = p.id
    order by vr.created_at desc, vr.id desc
    limit 1
), '') as latest_source,
coalesce((
    select vr.categories::text
    from content_violation_records vr
    where vr.post_id = p.id
    order by vr.created_at desc, vr.id desc
    limit 1
), '[]') as latest_categories,
(
    select ur.ends_at
    from user_restrictions ur
    where ur.user_id = p.author_user_id
      and ur.scope = 'post_publish'
      and ur.status = 'active'
      and (ur.ends_at is null or ur.ends_at > now())
    order by ur.ends_at desc nulls last, ur.id desc
    limit 1
) as post_mute_until`).
		Order("p.created_at desc").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&rows).Error; err != nil {
		return PostListResult{}, err
	}

	items := make([]PostListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, PostListItem{
			PostID:            row.PostID,
			AuthorUserID:      row.AuthorUserID,
			AuthorAccount:     row.AuthorAccount,
			AuthorNickname:    row.AuthorNickname,
			ContentText:       row.ContentText,
			Status:            row.Status,
			CampusCode:        normalizeCampusCode(row.CampusCode),
			Visibility:        row.Visibility,
			LikeCount:         row.LikeCount,
			CommentCount:      row.CommentCount,
			FavoriteCount:     row.FavoriteCount,
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
			LatestViolationAt: row.LatestViolationAt,
			LatestReason:      row.LatestReason,
			LatestSource:      row.LatestSource,
			LatestCategories:  parseStringArray(row.LatestCategories),
			PostMuteUntil:     row.PostMuteUntil,
		})
	}
	return PostListResult{Items: items, Total: total}, nil
}

func (r *gormRepository) RestorePostVisibility(ctx context.Context, postID int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Table("posts").
		Where("id = ? AND deleted_at IS NULL AND status = ?", postID, contentpb.PostStatus_POST_STATUS_BLOCKED).
		Updates(map[string]any{
			"status":     contentpb.PostStatus_POST_STATUS_NORMAL,
			"updated_at": gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *gormRepository) ListUserViolations(ctx context.Context, filter UserViolationFilter) (UserViolationResult, error) {
	normalizeViolationFilter(&filter)
	base := r.db.WithContext(ctx).Table("content_violation_records").Where("user_id = ?", filter.UserID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return UserViolationResult{}, err
	}

	var rows []violationRow
	if err := base.
		Select("violation_no, post_id, source, categories::text as categories, reason, mute_scope, mute_duration_seconds, coalesce(operator_user_id, 0) as operator_user_id, created_at").
		Order("created_at desc, id desc").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&rows).Error; err != nil {
		return UserViolationResult{}, err
	}

	items := make([]UserViolationItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, UserViolationItem{
			ViolationNo:         row.ViolationNo,
			PostID:              row.PostID,
			Source:              row.Source,
			Categories:          parseStringArray(row.Categories),
			Reason:              row.Reason,
			MuteScope:           row.MuteScope,
			MuteDurationSeconds: row.MuteDurationSeconds,
			OperatorUserID:      row.OperatorUserID,
			CreatedAt:           row.CreatedAt,
		})
	}
	return UserViolationResult{Items: items, Total: total}, nil
}

func defaultViolationCategories() []ViolationCategoryOption {
	return []ViolationCategoryOption{
		{Code: "色情", Label: "色情"},
		{Code: "政治敏感", Label: "政治敏感"},
		{Code: "暴恐违禁", Label: "暴恐违禁"},
		{Code: "广告引流", Label: "广告引流"},
		{Code: "辱骂不良", Label: "辱骂不良"},
		{Code: "其他", Label: "其他"},
	}
}

func parseStringArray(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	return items
}

func normalizePostListFilter(filter *PostListFilter) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.CampusCode = normalizeCampusCode(filter.CampusCode)
}

func normalizeViolationFilter(filter *UserViolationFilter) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
}

func normalizeCampusCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return "all"
	case "xinqiao":
		return "xinqiao"
	case "gaoji":
		return "gaoji"
	default:
		return "all"
	}
}

func requireDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("moderation: gorm db is required")
	}
	return db
}
