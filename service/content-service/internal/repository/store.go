package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"hfjj/backend/proto/contentpb"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Post struct {
	ID            int64
	AuthorUserID  int64
	ContentText   string
	ContentType   int32
	Topics        []string
	CampusCode    string
	Visibility    int32
	Status        int32
	LikeCount     int64
	CommentCount  int64
	FavoriteCount int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     sql.NullTime
	Media         []PostMedia
}

type UserContentStats struct {
	UserID            int64
	PostCount         int64
	ReceivedLikeCount int64
}

type PostMedia struct {
	ID         int64
	PostID     int64
	MediaType  int32
	StorageKey string
	URL        string
	Width      int64
	Height     int64
	DurationMS int64
	SizeBytes  int64
	SortOrder  int64
}

type Comment struct {
	ID               int64
	PostID           int64
	UserID           int64
	ReplyToCommentID int64
	ReplyToUserID    int64
	Content          string
	Status           int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        sql.NullTime
}

type PostStore interface {
	CreatePost(ctx context.Context, post Post) (Post, error)
	DeletePost(ctx context.Context, operatorUserID, postID int64, forceDelete bool) (bool, error)
	GetPost(ctx context.Context, postID int64) (Post, bool, error)
	ListPostsByAuthorIDs(ctx context.Context, operatorUserID int64, authorUserIDs []int64, page, pageSize int64) ([]Post, bool, error)
	ListDiscoverPosts(ctx context.Context, operatorUserID, page, pageSize int64) ([]Post, bool, error)
	SearchPosts(ctx context.Context, query string, page, pageSize int64) ([]Post, bool, error)
	GetUserStats(ctx context.Context, userID int64) (UserContentStats, error)
	CountPostsSince(ctx context.Context, userID int64, excludedStatus int32, since time.Time) (int64, error)
	IncrementCommentCount(ctx context.Context, postID int64, delta int64) error
	IncrementLikeCount(ctx context.Context, postID int64, delta int64) error
	IncrementFavoriteCount(ctx context.Context, postID int64, delta int64) error
	UpdatePostStatus(ctx context.Context, postID int64, status int32) error
}

type MembershipStore interface {
	ActivePlan(ctx context.Context, userID int64) (string, bool, error)
}

type CommentStore interface {
	CreateComment(ctx context.Context, comment Comment) (Comment, error)
	ListCommentsByPostID(ctx context.Context, postID, page, pageSize int64) ([]Comment, bool, error)
	GetComment(ctx context.Context, commentID int64) (Comment, bool, error)
	DeleteComment(ctx context.Context, operatorUserID, commentID int64) (bool, error)
}

type PostLikeStore interface {
	Like(ctx context.Context, userID, postID int64) (bool, error)
	Unlike(ctx context.Context, userID, postID int64) (bool, error)
}

type PostFavoriteStore interface {
	Favorite(ctx context.Context, userID, postID int64) (bool, error)
	Unfavorite(ctx context.Context, userID, postID int64) (bool, error)
}

type postStore struct {
	db *gorm.DB
}

type commentStore struct {
	db *gorm.DB
}

type postLikeStore struct {
	db *gorm.DB
}

type postFavoriteStore struct {
	db *gorm.DB
}

type membershipStore struct {
	db *gorm.DB
}

type contentPostRecord struct {
	ID            int64          `gorm:"column:id;primaryKey"`
	AuthorUserID  int64          `gorm:"column:author_user_id"`
	ContentText   string         `gorm:"column:content_text"`
	ContentType   int32          `gorm:"column:content_type"`
	CampusCode    string         `gorm:"column:campus_code"`
	Visibility    int32          `gorm:"column:visibility"`
	Status        int32          `gorm:"column:status"`
	LikeCount     int64          `gorm:"column:like_count"`
	CommentCount  int64          `gorm:"column:comment_count"`
	FavoriteCount int64          `gorm:"column:favorite_count"`
	CreatedAt     time.Time      `gorm:"column:created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (contentPostRecord) TableName() string {
	return "posts"
}

type contentPostMediaRecord struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	PostID     int64     `gorm:"column:post_id"`
	MediaType  int32     `gorm:"column:media_type"`
	StorageKey string    `gorm:"column:storage_key"`
	URL        string    `gorm:"column:url"`
	Width      int64     `gorm:"column:width"`
	Height     int64     `gorm:"column:height"`
	DurationMS int64     `gorm:"column:duration_ms"`
	SizeBytes  int64     `gorm:"column:size_bytes"`
	SortOrder  int64     `gorm:"column:sort_order"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (contentPostMediaRecord) TableName() string {
	return "post_media"
}

type contentCommentRecord struct {
	ID               int64          `gorm:"column:id;primaryKey"`
	PostID           int64          `gorm:"column:post_id"`
	UserID           int64          `gorm:"column:user_id"`
	ReplyToCommentID sql.NullInt64  `gorm:"column:reply_to_comment_id"`
	ReplyToUserID    sql.NullInt64  `gorm:"column:reply_to_user_id"`
	Content          string         `gorm:"column:content"`
	Status           int32          `gorm:"column:status"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (contentCommentRecord) TableName() string {
	return "comments"
}

type contentUserMembershipRecord struct {
	UserID   int64     `gorm:"column:user_id"`
	PlanCode string    `gorm:"column:plan_code"`
	StartsAt time.Time `gorm:"column:starts_at"`
	EndsAt   time.Time `gorm:"column:ends_at"`
}

func (contentUserMembershipRecord) TableName() string {
	return "user_memberships"
}

type contentPostLikeRecord struct {
	PostID    int64     `gorm:"column:post_id;primaryKey"`
	UserID    int64     `gorm:"column:user_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (contentPostLikeRecord) TableName() string {
	return "post_likes"
}

type contentPostFavoriteRecord struct {
	PostID    int64     `gorm:"column:post_id;primaryKey"`
	UserID    int64     `gorm:"column:user_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (contentPostFavoriteRecord) TableName() string {
	return "post_favorites"
}

func NewPostStore(db *gorm.DB) PostStore {
	return &postStore{db: requireGormDB(db)}
}

func NewMembershipStore(db *gorm.DB) MembershipStore {
	return &membershipStore{db: requireGormDB(db)}
}

func NewCommentStore(db *gorm.DB) CommentStore {
	return &commentStore{db: requireGormDB(db)}
}

func NewPostLikeStore(db *gorm.DB) PostLikeStore {
	return &postLikeStore{db: requireGormDB(db)}
}

func NewPostFavoriteStore(db *gorm.DB) PostFavoriteStore {
	return &postFavoriteStore{db: requireGormDB(db)}
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("content repository: gorm db is required")
	}
	return db
}

func (s *postStore) CreatePost(ctx context.Context, post Post) (Post, error) {
	post.CreatedAt = time.Now()
	post.UpdatedAt = post.CreatedAt
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := contentPostRecord{
			AuthorUserID:  post.AuthorUserID,
			ContentText:   post.ContentText,
			ContentType:   post.ContentType,
			CampusCode:    post.CampusCode,
			Visibility:    post.Visibility,
			Status:        post.Status,
			LikeCount:     post.LikeCount,
			CommentCount:  post.CommentCount,
			FavoriteCount: post.FavoriteCount,
			CreatedAt:     post.CreatedAt,
			UpdatedAt:     post.UpdatedAt,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		post.ID = record.ID
		post.CreatedAt = record.CreatedAt
		post.UpdatedAt = record.UpdatedAt

		for index := range post.Media {
			media := &post.Media[index]
			record := contentPostMediaRecord{
				PostID:     post.ID,
				MediaType:  media.MediaType,
				StorageKey: media.StorageKey,
				URL:        media.URL,
				Width:      media.Width,
				Height:     media.Height,
				DurationMS: media.DurationMS,
				SizeBytes:  media.SizeBytes,
				SortOrder:  media.SortOrder,
				CreatedAt:  time.Now(),
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			media.ID = record.ID
			media.PostID = post.ID
		}
		return nil
	})
	if err != nil {
		return Post{}, err
	}
	return post, nil
}

func (s *postStore) DeletePost(ctx context.Context, operatorUserID, postID int64, forceDelete bool) (bool, error) {
	query := s.db.WithContext(ctx).
		Model(&contentPostRecord{}).
		Where("id = ? AND deleted_at IS NULL", postID)
	if !forceDelete {
		query = query.Where("author_user_id = ?", operatorUserID)
	}
	result := query.Updates(map[string]any{
		"status":     contentpb.PostStatus_POST_STATUS_DELETED,
		"deleted_at": gorm.Expr("NOW()"),
		"updated_at": gorm.Expr("NOW()"),
	})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *postStore) GetPost(ctx context.Context, postID int64) (Post, bool, error) {
	var record contentPostRecord
	err := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", postID).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Post{}, false, nil
	}
	if err != nil {
		return Post{}, false, err
	}
	post := mapPostRecord(record)
	media, err := s.listMediaByPostID(ctx, post.ID)
	if err != nil {
		return Post{}, false, err
	}
	post.Media = media
	return post, true, nil
}

func (s *postStore) ListPostsByAuthorIDs(ctx context.Context, operatorUserID int64, authorUserIDs []int64, page, pageSize int64) ([]Post, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	if len(authorUserIDs) == 0 {
		return []Post{}, false, nil
	}
	var rows []contentPostRecord
	err := s.db.WithContext(ctx).
		Where("author_user_id IN ? AND deleted_at IS NULL", authorUserIDs).
		Where(
			"(status = ?) OR (author_user_id = ? AND status IN ?)",
			contentpb.PostStatus_POST_STATUS_NORMAL,
			operatorUserID,
			[]any{contentpb.PostStatus_POST_STATUS_REVIEWING, contentpb.PostStatus_POST_STATUS_BLOCKED},
		).
		Order("created_at DESC, id DESC").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	return s.buildPostsFromRecords(ctx, rows, limit)
}

func (s *postStore) ListDiscoverPosts(ctx context.Context, operatorUserID, page, pageSize int64) ([]Post, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	var rows []contentPostRecord
	err := s.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Where(
			"((status = ? AND visibility = ?) OR (author_user_id = ? AND status IN ?))",
			contentpb.PostStatus_POST_STATUS_NORMAL,
			contentpb.PostVisibility_POST_VISIBILITY_PUBLIC,
			operatorUserID,
			[]any{contentpb.PostStatus_POST_STATUS_NORMAL, contentpb.PostStatus_POST_STATUS_REVIEWING, contentpb.PostStatus_POST_STATUS_BLOCKED},
		).
		Order("created_at DESC, id DESC").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	return s.buildPostsFromRecords(ctx, rows, limit)
}

func (s *postStore) GetUserStats(ctx context.Context, userID int64) (UserContentStats, error) {
	stats := UserContentStats{UserID: userID}
	type row struct {
		PostCount         int64 `gorm:"column:post_count"`
		ReceivedLikeCount int64 `gorm:"column:received_like_count"`
	}
	var result row
	err := s.db.WithContext(ctx).Model(&contentPostRecord{}).
		Select("COUNT(*) AS post_count, COALESCE(SUM(like_count), 0) AS received_like_count").
		Where("author_user_id = ? AND status = ? AND deleted_at IS NULL", userID, contentpb.PostStatus_POST_STATUS_NORMAL).
		Scan(&result).Error
	if err != nil {
		return UserContentStats{}, err
	}
	stats.PostCount = result.PostCount
	stats.ReceivedLikeCount = result.ReceivedLikeCount
	return stats, nil
}

func (s *postStore) CountPostsSince(ctx context.Context, userID int64, excludedStatus int32, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&contentPostRecord{}).
		Where("author_user_id = ? AND status <> ? AND deleted_at IS NULL AND created_at >= ?", userID, excludedStatus, since).
		Count(&count).Error
	return count, err
}

func (s *membershipStore) ActivePlan(ctx context.Context, userID int64) (string, bool, error) {
	if userID <= 0 {
		return "", false, nil
	}

	var membership contentUserMembershipRecord
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND starts_at <= ? AND ends_at > ?", userID, time.Now(), time.Now()).
		Order("ends_at DESC").
		Take(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	planCode := strings.TrimSpace(membership.PlanCode)
	return planCode, planCode != "", nil
}

func (s *postStore) SearchPosts(ctx context.Context, query string, page, pageSize int64) ([]Post, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	normalizedQuery := strings.TrimSpace(strings.ToLower(query))
	if normalizedQuery == "" {
		return []Post{}, false, nil
	}

	escapedQuery := escapeSQLLikePattern(query)
	var rows []contentPostRecord
	err := s.db.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL", contentpb.PostStatus_POST_STATUS_NORMAL).
		Where("content_text ILIKE '%' || ? || '%' ESCAPE '\\'", escapedQuery).
		Order(gorm.Expr("CASE WHEN lower(content_text) LIKE lower(?) || '%' ESCAPE '\\' THEN 0 ELSE 1 END", escapedQuery)).
		Order("like_count DESC, comment_count DESC, created_at DESC").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	return s.buildPostsFromRecords(ctx, rows, limit)
}

func escapeSQLLikePattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "\\", "\\\\")
	pattern = strings.ReplaceAll(pattern, "%", "\\%")
	pattern = strings.ReplaceAll(pattern, "_", "\\_")
	return pattern
}

func (s *postStore) IncrementCommentCount(ctx context.Context, postID int64, delta int64) error {
	return s.incrementCounter(ctx, "comment_count", postID, delta)
}

func (s *postStore) IncrementLikeCount(ctx context.Context, postID int64, delta int64) error {
	return s.incrementCounter(ctx, "like_count", postID, delta)
}

func (s *postStore) IncrementFavoriteCount(ctx context.Context, postID int64, delta int64) error {
	return s.incrementCounter(ctx, "favorite_count", postID, delta)
}

func (s *postStore) UpdatePostStatus(ctx context.Context, postID int64, status int32) error {
	return s.db.WithContext(ctx).
		Model(&contentPostRecord{}).
		Where("id = ? AND deleted_at IS NULL", postID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

func (s *postStore) incrementCounter(ctx context.Context, field string, postID int64, delta int64) error {
	return s.db.WithContext(ctx).
		Model(&contentPostRecord{}).
		Where("id = ? AND deleted_at IS NULL", postID).
		Update(field, gorm.Expr(fmt.Sprintf("GREATEST(0, %s + ?)", field), delta)).Error
}

func (s *postStore) listMediaByPostID(ctx context.Context, postID int64) ([]PostMedia, error) {
	var rows []contentPostMediaRecord
	if err := s.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	media := make([]PostMedia, 0, len(rows))
	for _, row := range rows {
		media = append(media, mapPostMediaRecord(row))
	}
	return media, nil
}

func (s *postStore) buildPostsFromRecords(ctx context.Context, rows []contentPostRecord, limit int64) ([]Post, bool, error) {
	selectedRows := rows
	hasMore := false
	if int64(len(selectedRows)) > limit {
		selectedRows = selectedRows[:limit]
		hasMore = true
	}

	postIDs := make([]int64, 0, len(selectedRows))
	for _, row := range selectedRows {
		postIDs = append(postIDs, row.ID)
	}

	mediaByPostID, err := s.listMediaByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, false, err
	}

	posts := make([]Post, 0, len(selectedRows))
	for _, row := range selectedRows {
		post := mapPostRecord(row)
		post.Media = mediaByPostID[post.ID]
		posts = append(posts, post)
	}
	return posts, hasMore, nil
}

func (s *postStore) listMediaByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]PostMedia, error) {
	result := make(map[int64][]PostMedia, len(postIDs))
	if len(postIDs) == 0 {
		return result, nil
	}

	var rows []contentPostMediaRecord
	if err := s.db.WithContext(ctx).
		Where("post_id IN ?", postIDs).
		Order("post_id ASC, sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.PostID] = append(result[row.PostID], mapPostMediaRecord(row))
	}
	return result, nil
}

func (s *commentStore) CreateComment(ctx context.Context, comment Comment) (Comment, error) {
	comment.CreatedAt = time.Now()
	comment.UpdatedAt = comment.CreatedAt
	comment.Status = 1
	record := contentCommentRecord{
		PostID:           comment.PostID,
		UserID:           comment.UserID,
		ReplyToCommentID: contentNullInt64(comment.ReplyToCommentID),
		ReplyToUserID:    contentNullInt64(comment.ReplyToUserID),
		Content:          comment.Content,
		Status:           comment.Status,
		CreatedAt:        comment.CreatedAt,
		UpdatedAt:        comment.UpdatedAt,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return Comment{}, err
	}
	comment.ID = record.ID
	comment.CreatedAt = record.CreatedAt
	comment.UpdatedAt = record.UpdatedAt
	return comment, nil
}

func (s *commentStore) ListCommentsByPostID(ctx context.Context, postID, page, pageSize int64) ([]Comment, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	var rows []contentCommentRecord
	err := s.db.WithContext(ctx).
		Where("post_id = ? AND deleted_at IS NULL", postID).
		Order("created_at DESC, id DESC").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}

	comments := make([]Comment, 0, contentMinInt(len(rows), int(limit)))
	var hasMore bool
	for index, row := range rows {
		if int64(index) >= limit {
			hasMore = true
			break
		}
		comments = append(comments, mapCommentRecord(row))
	}
	return comments, hasMore, nil
}

func (s *commentStore) GetComment(ctx context.Context, commentID int64) (Comment, bool, error) {
	var record contentCommentRecord
	err := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", commentID).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Comment{}, false, nil
	}
	if err != nil {
		return Comment{}, false, err
	}
	return mapCommentRecord(record), true, nil
}

func (s *commentStore) DeleteComment(ctx context.Context, operatorUserID, commentID int64) (bool, error) {
	result := s.db.WithContext(ctx).
		Model(&contentCommentRecord{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", commentID, operatorUserID).
		Updates(map[string]any{
			"status":     3,
			"deleted_at": gorm.Expr("NOW()"),
			"updated_at": gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *postLikeStore) Like(ctx context.Context, userID, postID int64) (bool, error) {
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&contentPostLikeRecord{
		PostID:    postID,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *postLikeStore) Unlike(ctx context.Context, userID, postID int64) (bool, error) {
	result := s.db.WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).Delete(&contentPostLikeRecord{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *postFavoriteStore) Favorite(ctx context.Context, userID, postID int64) (bool, error) {
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&contentPostFavoriteRecord{
		PostID:    postID,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *postFavoriteStore) Unfavorite(ctx context.Context, userID, postID int64) (bool, error) {
	result := s.db.WithContext(ctx).Where("post_id = ? AND user_id = ?", postID, userID).Delete(&contentPostFavoriteRecord{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
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

func mapPostRecord(record contentPostRecord) Post {
	post := Post{
		ID:            record.ID,
		AuthorUserID:  record.AuthorUserID,
		ContentText:   record.ContentText,
		ContentType:   record.ContentType,
		CampusCode:    record.CampusCode,
		Visibility:    record.Visibility,
		Status:        record.Status,
		LikeCount:     record.LikeCount,
		CommentCount:  record.CommentCount,
		FavoriteCount: record.FavoriteCount,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
	if record.DeletedAt.Valid {
		post.DeletedAt = sql.NullTime{Time: record.DeletedAt.Time, Valid: true}
	}
	return post
}

func mapPostMediaRecord(record contentPostMediaRecord) PostMedia {
	return PostMedia{
		ID:         record.ID,
		PostID:     record.PostID,
		MediaType:  record.MediaType,
		StorageKey: record.StorageKey,
		URL:        record.URL,
		Width:      record.Width,
		Height:     record.Height,
		DurationMS: record.DurationMS,
		SizeBytes:  record.SizeBytes,
		SortOrder:  record.SortOrder,
	}
}

func mapCommentRecord(record contentCommentRecord) Comment {
	comment := Comment{
		ID:        record.ID,
		PostID:    record.PostID,
		UserID:    record.UserID,
		Content:   record.Content,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
	if record.ReplyToCommentID.Valid {
		comment.ReplyToCommentID = record.ReplyToCommentID.Int64
	}
	if record.ReplyToUserID.Valid {
		comment.ReplyToUserID = record.ReplyToUserID.Int64
	}
	if record.DeletedAt.Valid {
		comment.DeletedAt = sql.NullTime{Time: record.DeletedAt.Time, Valid: true}
	}
	return comment
}

func contentNullInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func contentMinInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
