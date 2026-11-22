package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"hfjj/backend/proto/contentpb"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const topicPageLookahead int64 = 1

type Topic struct {
	ID              int64
	TopicName       string
	DisplayName     string
	PostCount       int64
	HotScore        float64
	LastRefreshedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TopicDraft struct {
	TopicName   string
	DisplayName string
}

type HotTopic struct {
	TopicID         int64
	RankNo          int32
	TopicName       string
	DisplayName     string
	PostCount       int64
	HotScore        float64
	CalculatedAt    time.Time
	LastRefreshedAt time.Time
}

type TopicStore interface {
	BindPostTopics(ctx context.Context, postID int64, drafts []TopicDraft) ([]Topic, error)
	ListTopicsByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]Topic, error)
	GetTopic(ctx context.Context, topicID int64) (Topic, bool, error)
	ListPostsByTopicID(ctx context.Context, topicID, page, pageSize int64) ([]Post, bool, error)
	SearchTopics(ctx context.Context, query string, page, pageSize int64) ([]Topic, bool, error)
	ListHotTopics(ctx context.Context, limit int64) ([]HotTopic, error)
	BuildHotTopics(ctx context.Context, limit int64) ([]HotTopic, error)
	ReplaceHotTopics(ctx context.Context, topics []HotTopic) error
}

type topicStore struct {
	db *gorm.DB
}

type topicRecord struct {
	ID              int64          `gorm:"column:id;primaryKey"`
	TopicName       string         `gorm:"column:topic_name"`
	DisplayName     string         `gorm:"column:display_name"`
	PostCount       int64          `gorm:"column:post_count"`
	HotScore        float64        `gorm:"column:hot_score"`
	LastRefreshedAt *time.Time     `gorm:"column:last_refreshed_at"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (topicRecord) TableName() string {
	return "topics"
}

type postTopicRecord struct {
	PostID    int64     `gorm:"column:post_id;primaryKey"`
	TopicID   int64     `gorm:"column:topic_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (postTopicRecord) TableName() string {
	return "post_topics"
}

type hotTopicRecord struct {
	TopicID      int64     `gorm:"column:topic_id;primaryKey"`
	RankNo       int32     `gorm:"column:rank_no"`
	HotScore     float64   `gorm:"column:hot_score"`
	PostCount    int64     `gorm:"column:post_count"`
	CalculatedAt time.Time `gorm:"column:calculated_at"`
}

func (hotTopicRecord) TableName() string {
	return "hot_topics"
}

type contentTopicPostRecord struct {
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

func (contentTopicPostRecord) TableName() string {
	return "posts"
}

func NewTopicStore(db *gorm.DB) TopicStore {
	return &topicStore{db: requireGormDB(db)}
}

func (s *topicStore) BindPostTopics(ctx context.Context, postID int64, drafts []TopicDraft) ([]Topic, error) {
	unique := make([]TopicDraft, 0, len(drafts))
	seen := make(map[string]struct{}, len(drafts))
	for _, draft := range drafts {
		topicName := normalizeTopicName(draft.TopicName)
		displayName := strings.TrimSpace(draft.DisplayName)
		if topicName == "" || displayName == "" {
			continue
		}
		if _, ok := seen[topicName]; ok {
			continue
		}
		seen[topicName] = struct{}{}
		unique = append(unique, TopicDraft{
			TopicName:   topicName,
			DisplayName: displayName,
		})
	}
	if len(unique) == 0 || postID <= 0 {
		return []Topic{}, nil
	}

	bound := make([]Topic, 0, len(unique))
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for _, draft := range unique {
			record := topicRecord{
				TopicName:   draft.TopicName,
				DisplayName: draft.DisplayName,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "topic_name"}},
				DoUpdates: clause.Assignments(map[string]any{
					"display_name": draft.DisplayName,
					"updated_at":   gorm.Expr("NOW()"),
				}),
			}).Create(&record).Error; err != nil {
				return err
			}

			if err := tx.Where("topic_name = ? AND deleted_at IS NULL", draft.TopicName).Take(&record).Error; err != nil {
				return err
			}

			link := postTopicRecord{
				PostID:    postID,
				TopicID:   record.ID,
				CreatedAt: now,
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := tx.Model(&topicRecord{}).
					Where("id = ?", record.ID).
					Updates(map[string]any{
						"post_count": gorm.Expr("post_count + 1"),
						"updated_at": gorm.Expr("NOW()"),
					}).Error; err != nil {
					return err
				}
				record.PostCount++
				record.UpdatedAt = time.Now()
			}
			bound = append(bound, mapTopicRecord(record))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return bound, nil
}

func (s *topicStore) ListTopicsByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]Topic, error) {
	result := make(map[int64][]Topic, len(postIDs))
	if len(postIDs) == 0 {
		return result, nil
	}

	type row struct {
		PostID int64 `gorm:"column:post_id"`
		topicRecord
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("post_topics AS pt").
		Select("pt.post_id, t.id, t.topic_name, t.display_name, t.post_count, t.hot_score, t.last_refreshed_at, t.created_at, t.updated_at").
		Joins("JOIN topics t ON t.id = pt.topic_id").
		Where("pt.post_id IN ? AND t.deleted_at IS NULL", postIDs).
		Order("pt.post_id ASC, t.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.PostID] = append(result[row.PostID], mapTopicRecord(row.topicRecord))
	}
	return result, nil
}

func (s *topicStore) GetTopic(ctx context.Context, topicID int64) (Topic, bool, error) {
	if topicID <= 0 {
		return Topic{}, false, nil
	}

	var record topicRecord
	err := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", topicID).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Topic{}, false, nil
	}
	if err != nil {
		return Topic{}, false, err
	}
	return mapTopicRecord(record), true, nil
}

func (s *topicStore) ListPostsByTopicID(ctx context.Context, topicID, page, pageSize int64) ([]Post, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	if topicID <= 0 {
		return []Post{}, false, nil
	}

	var rows []contentTopicPostRecord
	err := s.db.WithContext(ctx).
		Table("post_topics AS pt").
		Select("p.id, p.author_user_id, p.content_text, p.content_type, p.campus_code, p.visibility, p.status, p.like_count, p.comment_count, p.favorite_count, p.created_at, p.updated_at, p.deleted_at").
		Joins("JOIN posts p ON p.id = pt.post_id").
		Where("pt.topic_id = ? AND p.deleted_at IS NULL AND p.status = ?", topicID, contentpb.PostStatus_POST_STATUS_NORMAL).
		Order("p.like_count DESC, p.comment_count DESC, p.created_at DESC").
		Offset(int(offset)).
		Limit(int(limit + topicPageLookahead)).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}

	posts := make([]Post, 0, contentMinInt(len(rows), int(limit)))
	var hasMore bool
	for index, row := range rows {
		if int64(index) >= limit {
			hasMore = true
			break
		}
		posts = append(posts, mapTopicPostRecord(row))
	}
	return posts, hasMore, nil
}

func (s *topicStore) SearchTopics(ctx context.Context, query string, page, pageSize int64) ([]Topic, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)
	normalizedQuery := strings.TrimSpace(strings.ToLower(query))
	if normalizedQuery == "" {
		return []Topic{}, false, nil
	}

	escapedQuery := escapeSQLLikePattern(query)
	var rows []topicRecord
	err := s.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Where("(display_name ILIKE '%' || ? || '%' ESCAPE '\\' OR topic_name ILIKE '%' || ? || '%' ESCAPE '\\')", escapedQuery, escapedQuery).
		Order("hot_score DESC, post_count DESC, id DESC").
		Offset(int(offset)).
		Limit(int(limit + topicPageLookahead)).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}

	items := make([]Topic, 0, contentMinInt(len(rows), int(limit)))
	var hasMore bool
	for index, row := range rows {
		if int64(index) >= limit {
			hasMore = true
			break
		}
		items = append(items, mapTopicRecord(row))
	}
	return items, hasMore, nil
}

func (s *topicStore) ListHotTopics(ctx context.Context, limit int64) ([]HotTopic, error) {
	if limit <= 0 {
		limit = 10
	}

	type row struct {
		TopicID         int64      `gorm:"column:topic_id"`
		RankNo          int32      `gorm:"column:rank_no"`
		TopicName       string     `gorm:"column:topic_name"`
		DisplayName     string     `gorm:"column:display_name"`
		PostCount       int64      `gorm:"column:post_count"`
		HotScore        float64    `gorm:"column:hot_score"`
		CalculatedAt    time.Time  `gorm:"column:calculated_at"`
		LastRefreshedAt *time.Time `gorm:"column:last_refreshed_at"`
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("hot_topics AS ht").
		Select("ht.topic_id, ht.rank_no, t.topic_name, t.display_name, ht.post_count, ht.hot_score, ht.calculated_at, t.last_refreshed_at").
		Joins("JOIN topics t ON t.id = ht.topic_id").
		Where("t.deleted_at IS NULL").
		Order("ht.rank_no ASC").
		Limit(int(limit)).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]HotTopic, 0, len(rows))
	for _, row := range rows {
		item := HotTopic{
			TopicID:      row.TopicID,
			RankNo:       row.RankNo,
			TopicName:    row.TopicName,
			DisplayName:  row.DisplayName,
			PostCount:    row.PostCount,
			HotScore:     row.HotScore,
			CalculatedAt: row.CalculatedAt,
		}
		if row.LastRefreshedAt != nil {
			item.LastRefreshedAt = *row.LastRefreshedAt
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *topicStore) BuildHotTopics(ctx context.Context, limit int64) ([]HotTopic, error) {
	if limit <= 0 {
		limit = 10
	}

	type metricRow struct {
		TopicID     int64   `gorm:"column:topic_id"`
		TopicName   string  `gorm:"column:topic_name"`
		DisplayName string  `gorm:"column:display_name"`
		PostCount   int64   `gorm:"column:post_count"`
		HotScore    float64 `gorm:"column:hot_score"`
	}
	var rows []metricRow
	err := s.db.WithContext(ctx).
		Table("topics AS t").
		Select(`
t.id AS topic_id,
t.topic_name,
t.display_name,
COUNT(pt.post_id)::bigint AS post_count,
SUM(
    1
    + LEAST(p.like_count, 200) * 0.45
    + LEAST(p.comment_count, 200) * 0.65
    + LEAST(p.favorite_count, 200) * 0.35
)::double precision AS hot_score`).
		Joins("JOIN post_topics pt ON pt.topic_id = t.id").
		Joins("JOIN posts p ON p.id = pt.post_id").
		Where("t.deleted_at IS NULL AND p.deleted_at IS NULL AND p.status = ? AND p.created_at >= ?", contentpb.PostStatus_POST_STATUS_NORMAL, time.Now().AddDate(0, 0, -30)).
		Group("t.id, t.topic_name, t.display_name").
		Order("hot_score DESC, post_count DESC, topic_id ASC").
		Limit(int(limit)).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	now := time.Now()
	items := make([]HotTopic, 0, len(rows))
	for _, row := range rows {
		items = append(items, HotTopic{
			TopicID:         row.TopicID,
			RankNo:          int32(len(items) + 1),
			TopicName:       row.TopicName,
			DisplayName:     row.DisplayName,
			PostCount:       row.PostCount,
			HotScore:        row.HotScore,
			CalculatedAt:    now,
			LastRefreshedAt: now,
		})
	}
	return items, nil
}

func (s *topicStore) ReplaceHotTopics(ctx context.Context, topics []HotTopic) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&hotTopicRecord{}).Error; err != nil {
			return err
		}
		for _, item := range topics {
			if err := tx.Create(&hotTopicRecord{
				TopicID:      item.TopicID,
				RankNo:       item.RankNo,
				HotScore:     item.HotScore,
				PostCount:    item.PostCount,
				CalculatedAt: item.CalculatedAt,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&topicRecord{}).
				Where("id = ?", item.TopicID).
				Updates(map[string]any{
					"hot_score":         item.HotScore,
					"last_refreshed_at": item.LastRefreshedAt,
					"updated_at":        gorm.Expr("NOW()"),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeTopicName(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "#")
	trimmed = strings.TrimSuffix(trimmed, "#")
	return strings.ToLower(strings.TrimSpace(trimmed))
}

func mapTopicRecord(record topicRecord) Topic {
	topic := Topic{
		ID:          record.ID,
		TopicName:   record.TopicName,
		DisplayName: record.DisplayName,
		PostCount:   record.PostCount,
		HotScore:    record.HotScore,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	if record.LastRefreshedAt != nil {
		topic.LastRefreshedAt = *record.LastRefreshedAt
	}
	return topic
}

func mapTopicPostRecord(record contentTopicPostRecord) Post {
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
