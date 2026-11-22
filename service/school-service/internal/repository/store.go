package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hfjj/backend/school-service/internal/types"
)

type AuthStore interface {
	Set(ctx context.Context, key string, value types.PreAuthSession) error
	Get(ctx context.Context, key string) (types.PreAuthSession, bool, error)
	Delete(ctx context.Context, key string) error
}

type AuthorizedStore interface {
	Set(ctx context.Context, key string, value types.AuthorizedTicket) error
	Get(ctx context.Context, key string) (types.AuthorizedTicket, bool, error)
	Delete(ctx context.Context, key string) error
}

type SchoolAccountStore interface {
	Save(ctx context.Context, account types.SchoolAccount) (types.SchoolAccount, error)
	GetByUserID(ctx context.Context, userID int64) (types.SchoolAccount, bool, error)
	GetByStudentNo(ctx context.Context, studentNo string) (types.SchoolAccount, bool, error)
	ReconcileStudentStatuses(ctx context.Context, now time.Time) (int64, error)
}

type SchoolSessionStore interface {
	Set(ctx context.Context, session types.SchoolSession) error
	Get(ctx context.Context, userID int64) (types.SchoolSession, bool, error)
	Delete(ctx context.Context, userID int64) error
}

type redisJSONStore[T any] struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func newRedisJSONStore[T any](client *redis.Client, prefix string, ttl time.Duration) *redisJSONStore[T] {
	return &redisJSONStore[T]{
		client: requireRedisClient(client),
		prefix: prefix,
		ttl:    ttl,
	}
}

func requireRedisClient(client *redis.Client) *redis.Client {
	if client == nil {
		panic("redis client is required")
	}
	return client
}

func (s *redisJSONStore[T]) set(ctx context.Context, key string, value T) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(storeContext(ctx), s.prefix+key, payload, s.ttl).Err()
}

func (s *redisJSONStore[T]) get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	raw, err := s.client.Get(storeContext(ctx), s.prefix+key).Result()
	if err == redis.Nil {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}

	var value T
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return zero, false, err
	}
	return value, true, nil
}

func (s *redisJSONStore[T]) delete(ctx context.Context, key string) error {
	return s.client.Del(storeContext(ctx), s.prefix+key).Err()
}

type authStore struct {
	store *redisJSONStore[types.PreAuthSession]
}

func NewAuthStore(client *redis.Client, prefix string) AuthStore {
	return &authStore{store: newRedisJSONStore[types.PreAuthSession](client, prefix+"preauth:", 10*time.Minute)}
}

func (s *authStore) Set(ctx context.Context, key string, value types.PreAuthSession) error {
	return s.store.set(ctx, key, value)
}

func (s *authStore) Get(ctx context.Context, key string) (types.PreAuthSession, bool, error) {
	return s.store.get(ctx, key)
}

func (s *authStore) Delete(ctx context.Context, key string) error {
	return s.store.delete(ctx, key)
}

type authorizedStore struct {
	store *redisJSONStore[types.AuthorizedTicket]
}

func NewAuthorizedStore(client *redis.Client, prefix string) AuthorizedStore {
	return &authorizedStore{store: newRedisJSONStore[types.AuthorizedTicket](client, prefix+"authorized:", 30*time.Minute)}
}

func (s *authorizedStore) Set(ctx context.Context, key string, value types.AuthorizedTicket) error {
	return s.store.set(ctx, key, value)
}

func (s *authorizedStore) Get(ctx context.Context, key string) (types.AuthorizedTicket, bool, error) {
	return s.store.get(ctx, key)
}

func (s *authorizedStore) Delete(ctx context.Context, key string) error {
	return s.store.delete(ctx, key)
}

type schoolAccountStore struct {
	db *gorm.DB
}

type schoolAccountRecord struct {
	ID            int64      `gorm:"column:id;primaryKey"`
	UserID        int64      `gorm:"column:user_id"`
	StudentNo     string     `gorm:"column:student_no"`
	StudentName   string     `gorm:"column:student_name"`
	Password      string     `gorm:"column:password"`
	StudentStatus int32      `gorm:"column:student_status"`
	BindStatus    int32      `gorm:"column:bind_status"`
	AdmissionAt   *time.Time `gorm:"column:admission_at"`
	GraduationAt  *time.Time `gorm:"column:graduation_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (schoolAccountRecord) TableName() string {
	return "school_accounts"
}

func NewSchoolAccountStore(db *gorm.DB) SchoolAccountStore {
	return &schoolAccountStore{db: requireGormDB(db)}
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("school repository: gorm db is required")
	}
	return db
}

func (s *schoolAccountStore) Save(ctx context.Context, account types.SchoolAccount) (types.SchoolAccount, error) {
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now()
	}
	if account.UpdatedAt.IsZero() {
		account.UpdatedAt = time.Now()
	}
	record := schoolAccountRecord{
		UserID:        account.UserID,
		StudentNo:     account.StudentNo,
		StudentName:   account.StudentName,
		Password:      account.Password,
		StudentStatus: account.StudentStatus,
		BindStatus:    account.BindStatus,
		AdmissionAt:   timePtrOrNil(account.AdmissionAt),
		GraduationAt:  timePtrOrNil(account.GraduationAt),
		CreatedAt:     account.CreatedAt,
		UpdatedAt:     account.UpdatedAt,
	}
	err := s.db.WithContext(storeContext(ctx)).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"student_no":     record.StudentNo,
			"student_name":   record.StudentName,
			"password":       record.Password,
			"student_status": record.StudentStatus,
			"bind_status":    record.BindStatus,
			"admission_at":   record.AdmissionAt,
			"graduation_at":  record.GraduationAt,
			"updated_at":     record.UpdatedAt,
		}),
	}).Create(&record).Error
	if err != nil {
		return types.SchoolAccount{}, err
	}
	account.ID = record.ID
	account.CreatedAt = record.CreatedAt
	account.UpdatedAt = record.UpdatedAt
	return account, nil
}

func (s *schoolAccountStore) GetByUserID(ctx context.Context, userID int64) (types.SchoolAccount, bool, error) {
	var record schoolAccountRecord
	err := s.db.WithContext(storeContext(ctx)).Where("user_id = ?", userID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return types.SchoolAccount{}, false, nil
	}
	if err != nil {
		return types.SchoolAccount{}, false, err
	}
	return mapSchoolAccountRecord(record), true, nil
}

func (s *schoolAccountStore) GetByStudentNo(ctx context.Context, studentNo string) (types.SchoolAccount, bool, error) {
	var record schoolAccountRecord
	err := s.db.WithContext(storeContext(ctx)).Where("student_no = ?", studentNo).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return types.SchoolAccount{}, false, nil
	}
	if err != nil {
		return types.SchoolAccount{}, false, err
	}
	return mapSchoolAccountRecord(record), true, nil
}

func (s *schoolAccountStore) ReconcileStudentStatuses(ctx context.Context, now time.Time) (int64, error) {
	var records []schoolAccountRecord
	if err := s.db.WithContext(storeContext(ctx)).
		Where("bind_status = ?", 1).
		Find(&records).Error; err != nil {
		return 0, err
	}
	var changed int64
	for _, record := range records {
		account := mapSchoolAccountRecord(record)
		nextStatus := reconcileStatusByDates(account, now)
		if nextStatus == record.StudentStatus {
			continue
		}
		if err := s.db.WithContext(storeContext(ctx)).
			Model(&schoolAccountRecord{}).
			Where("id = ?", record.ID).
			Updates(map[string]any{
				"student_status": nextStatus,
				"updated_at":     now,
			}).Error; err != nil {
			return changed, err
		}
		changed++
	}
	return changed, nil
}

type schoolSessionStore struct {
	store *redisJSONStore[types.SchoolSession]
}

func NewSchoolSessionStore(client *redis.Client, prefix string) SchoolSessionStore {
	return &schoolSessionStore{store: newRedisJSONStore[types.SchoolSession](client, prefix+"session:", 24*time.Hour)}
}

func (s *schoolSessionStore) Set(ctx context.Context, session types.SchoolSession) error {
	return s.store.set(ctx, strconv.FormatInt(session.UserID, 10), session)
}

func (s *schoolSessionStore) Get(ctx context.Context, userID int64) (types.SchoolSession, bool, error) {
	return s.store.get(ctx, strconv.FormatInt(userID, 10))
}

func (s *schoolSessionStore) Delete(ctx context.Context, userID int64) error {
	return s.store.delete(ctx, strconv.FormatInt(userID, 10))
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func timePtrOrNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

func mapSchoolAccountRecord(record schoolAccountRecord) types.SchoolAccount {
	account := types.SchoolAccount{
		ID:            record.ID,
		UserID:        record.UserID,
		StudentNo:     record.StudentNo,
		StudentName:   record.StudentName,
		Password:      record.Password,
		StudentStatus: record.StudentStatus,
		BindStatus:    record.BindStatus,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
	if record.AdmissionAt != nil {
		account.AdmissionAt = *record.AdmissionAt
	}
	if record.GraduationAt != nil {
		account.GraduationAt = *record.GraduationAt
	}
	return account
}

func storeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func wrapStoreErr(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s failed: %w", action, err)
}

func reconcileStatusByDates(account types.SchoolAccount, now time.Time) int32 {
	if account.BindStatus != 1 {
		return account.StudentStatus
	}
	if !account.GraduationAt.IsZero() && !account.GraduationAt.After(now) {
		return 2
	}
	if account.StudentStatus == 3 {
		return 3
	}
	if !account.AdmissionAt.IsZero() && !account.AdmissionAt.After(now) {
		return 1
	}
	if !account.AdmissionAt.IsZero() && account.AdmissionAt.After(now) {
		return 0
	}
	return account.StudentStatus
}
