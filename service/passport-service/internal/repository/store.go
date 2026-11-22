package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

type User struct {
	ID            int64
	Account       string
	Nickname      string
	PasswordHash  string
	AvatarURL     string
	CoverImageURL string
	Bio           string
	AccountStatus int32
	SchoolBound   bool
	LastLoginAt   time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type LoginRecord struct {
	UserID  int64
	LoginIP string
	Device  string
	LoginAt time.Time
}

type AccessToken struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RefreshToken struct {
	Token     string    `json:"token"`
	JTI       string    `json:"jti"`
	UserID    int64     `json:"user_id"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Session struct {
	SessionID  string    `json:"session_id"`
	UserID     int64     `json:"user_id"`
	Device     string    `json:"device"`
	LoginIP    string    `json:"login_ip"`
	AccessJTI  string    `json:"access_jti"`
	RefreshJTI string    `json:"refresh_jti"`
	ExpiresAt  time.Time `json:"expires_at"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UserRoleChangeLog struct {
	OperatorUserID int64
	TargetUserID   int64
	BeforeRoles    []string
	AfterRoles     []string
	CreatedAt      time.Time
}

type UserStore interface {
	Create(user User) (User, error)
	Delete(id int64) error
	Save(user User) error
	GetByID(id int64) (User, bool, error)
	GetByAccount(account string) (User, bool, error)
	GetByStudentNo(studentNo string) (User, bool, error)
	ExistsNickname(nickname string) (bool, error)
}

type LoginRecordStore interface {
	Create(record LoginRecord) error
}

type SessionStore interface {
	Put(session Session) error
	Get(sessionID string) (Session, bool, error)
	Delete(sessionID string) error
}

type UserRoleChangeLogStore interface {
	Create(record UserRoleChangeLog) error
}

type MembershipExpiryStore interface {
	ExpiredMembershipUserIDs(ctx context.Context) ([]int64, error)
	RemoveExpiredMembershipsAndRoles(ctx context.Context, userIDs []int64, membershipRoleCodes []string) error
}

type TokenStore interface {
	PutAccess(token AccessToken) error
	GetAccess(token string) (AccessToken, bool, error)
	DeleteAccess(token string) error
	PutRefresh(token RefreshToken) error
	GetRefresh(token string) (RefreshToken, bool, error)
	DeleteRefresh(token string) error
}

type userStore struct {
	db *gorm.DB
}

type userRecord struct {
	ID            int64          `gorm:"column:id;primaryKey"`
	Account       string         `gorm:"column:account"`
	Nickname      string         `gorm:"column:nickname"`
	PasswordHash  string         `gorm:"column:password_hash"`
	AvatarURL     string         `gorm:"column:avatar_url"`
	CoverImageURL string         `gorm:"column:cover_image_url"`
	Bio           string         `gorm:"column:bio"`
	AccountStatus int32          `gorm:"column:account_status"`
	SchoolBound   bool           `gorm:"column:school_bound"`
	LastLoginAt   *time.Time     `gorm:"column:last_login_at"`
	CreatedAt     time.Time      `gorm:"column:created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (userRecord) TableName() string {
	return "users"
}

type loginRecordEntity struct {
	UserID  int64     `gorm:"column:user_id"`
	LoginIP string    `gorm:"column:login_ip"`
	Device  string    `gorm:"column:device"`
	LoginAt time.Time `gorm:"column:login_at"`
}

func (loginRecordEntity) TableName() string {
	return "user_login_records"
}

type userRoleChangeLogEntity struct {
	OperatorUserID int64     `gorm:"column:operator_user_id"`
	TargetUserID   int64     `gorm:"column:target_user_id"`
	BeforeRoles    []byte    `gorm:"column:before_roles"`
	AfterRoles     []byte    `gorm:"column:after_roles"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (userRoleChangeLogEntity) TableName() string {
	return "user_role_change_logs"
}

type userMembershipRecord struct {
	UserID int64     `gorm:"column:user_id;primaryKey"`
	EndsAt time.Time `gorm:"column:ends_at"`
}

func (userMembershipRecord) TableName() string {
	return "user_memberships"
}

func NewUserStore(db *gorm.DB) UserStore {
	return &userStore{db: requireGormDB(db)}
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("passport repository: gorm db is required")
	}
	return db
}

func (s *userStore) Create(user User) (User, error) {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = user.CreatedAt
	}
	record := userRecord{
		Account:       user.Account,
		Nickname:      user.Nickname,
		PasswordHash:  user.PasswordHash,
		AvatarURL:     user.AvatarURL,
		CoverImageURL: user.CoverImageURL,
		Bio:           user.Bio,
		AccountStatus: user.AccountStatus,
		SchoolBound:   user.SchoolBound,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
	if err := s.db.WithContext(context.Background()).Create(&record).Error; err != nil {
		return User{}, err
	}
	user.ID = record.ID
	user.CreatedAt = record.CreatedAt
	user.UpdatedAt = record.UpdatedAt
	return user, nil
}

func (s *userStore) Delete(id int64) error {
	return s.db.WithContext(context.Background()).Delete(&userRecord{}, "id = ?", id).Error
}

func (s *userStore) Save(user User) error {
	user.UpdatedAt = time.Now()
	return s.db.WithContext(context.Background()).
		Model(&userRecord{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"nickname":        user.Nickname,
			"password_hash":   user.PasswordHash,
			"avatar_url":      user.AvatarURL,
			"cover_image_url": user.CoverImageURL,
			"bio":             user.Bio,
			"account_status":  user.AccountStatus,
			"school_bound":    user.SchoolBound,
			"last_login_at":   nullTime(user.LastLoginAt),
			"updated_at":      user.UpdatedAt,
		}).Error
}

func (s *userStore) GetByID(id int64) (User, bool, error) {
	var record userRecord
	err := s.db.WithContext(context.Background()).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return mapUserRecord(record), true, nil
}

func (s *userStore) GetByAccount(account string) (User, bool, error) {
	var record userRecord
	err := s.db.WithContext(context.Background()).
		Where("account = ? AND deleted_at IS NULL", account).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return mapUserRecord(record), true, nil
}

func (s *userStore) GetByStudentNo(studentNo string) (User, bool, error) {
	var record userRecord
	err := s.db.WithContext(context.Background()).
		Table("users AS u").
		Select("u.id, u.account, u.nickname, u.password_hash, u.avatar_url, u.cover_image_url, u.bio, u.account_status, u.school_bound, u.last_login_at, u.created_at, u.updated_at").
		Joins("JOIN school_accounts sa ON sa.user_id = u.id").
		Where("sa.student_no = ? AND u.deleted_at IS NULL AND sa.bind_status = 1", studentNo).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return mapUserRecord(record), true, nil
}

func (s *userStore) ExistsNickname(nickname string) (bool, error) {
	var count int64
	err := s.db.WithContext(context.Background()).
		Model(&userRecord{}).
		Where("nickname = ? AND deleted_at IS NULL", nickname).
		Count(&count).Error
	return count > 0, err
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

type loginRecordStore struct {
	db *gorm.DB
}

type userRoleChangeLogStore struct {
	db *gorm.DB
}

type membershipExpiryStore struct {
	db *gorm.DB
}

func NewLoginRecordStore(db *gorm.DB) LoginRecordStore {
	return &loginRecordStore{db: requireGormDB(db)}
}

// NewUserRoleChangeLogStore 创建角色变更审计仓储。
// 有数据库时写 PostgreSQL，无数据库时退化为内存记录，便于本地联调。
func NewUserRoleChangeLogStore(db *gorm.DB) UserRoleChangeLogStore {
	return &userRoleChangeLogStore{db: requireGormDB(db)}
}

func NewMembershipExpiryStore(db *gorm.DB) MembershipExpiryStore {
	return &membershipExpiryStore{db: requireGormDB(db)}
}

func (s *loginRecordStore) Create(record LoginRecord) error {
	if record.LoginAt.IsZero() {
		record.LoginAt = time.Now()
	}
	return s.db.WithContext(context.Background()).Create(&loginRecordEntity{
		UserID:  record.UserID,
		LoginIP: record.LoginIP,
		Device:  record.Device,
		LoginAt: record.LoginAt,
	}).Error
}

func (s *userRoleChangeLogStore) Create(record UserRoleChangeLog) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	beforeRoles, err := json.Marshal(record.BeforeRoles)
	if err != nil {
		return err
	}
	afterRoles, err := json.Marshal(record.AfterRoles)
	if err != nil {
		return err
	}

	return s.db.WithContext(context.Background()).Create(&userRoleChangeLogEntity{
		OperatorUserID: record.OperatorUserID,
		TargetUserID:   record.TargetUserID,
		BeforeRoles:    beforeRoles,
		AfterRoles:     afterRoles,
		CreatedAt:      record.CreatedAt,
	}).Error
}

func (s *membershipExpiryStore) ExpiredMembershipUserIDs(ctx context.Context) ([]int64, error) {
	var userIDs []int64
	err := s.db.WithContext(ctx).
		Table("user_memberships").
		Where("ends_at <= ?", time.Now()).
		Pluck("user_id", &userIDs).Error
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil, nil
		}
		return nil, err
	}
	filtered := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID > 0 {
			filtered = append(filtered, userID)
		}
	}
	return filtered, nil
}

func (s *membershipExpiryStore) RemoveExpiredMembershipsAndRoles(ctx context.Context, userIDs []int64, membershipRoleCodes []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, userID := range userIDs {
			subject := strconv.FormatInt(userID, 10)
			if len(membershipRoleCodes) > 0 {
				if err := tx.Where("ptype = ? and v0 = ? and v1 in ?", "g", subject, membershipRoleCodes).Delete(&casbinRule{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("user_id = ? AND ends_at <= ?", userID, time.Now()).Delete(&userMembershipRecord{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func mapUserRecord(record userRecord) User {
	user := User{
		ID:            record.ID,
		Account:       record.Account,
		Nickname:      record.Nickname,
		PasswordHash:  record.PasswordHash,
		AvatarURL:     record.AvatarURL,
		CoverImageURL: record.CoverImageURL,
		Bio:           record.Bio,
		AccountStatus: record.AccountStatus,
		SchoolBound:   record.SchoolBound,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
	if record.LastLoginAt != nil {
		user.LastLoginAt = *record.LastLoginAt
	}
	return user
}

type casbinRule struct {
	ID    uint   `gorm:"column:id;primaryKey"`
	PType string `gorm:"column:ptype"`
	V0    string `gorm:"column:v0"`
	V1    string `gorm:"column:v1"`
}

func (casbinRule) TableName() string {
	return "casbin_rule"
}

type tokenStore struct {
	client *redis.Client
	prefix string
}

func NewTokenStore(client *redis.Client, prefix string) TokenStore {
	return &tokenStore{
		client: requireRedisClient(client),
		prefix: prefix,
	}
}

func requireRedisClient(client *redis.Client) *redis.Client {
	if client == nil {
		panic("redis client is required")
	}
	return client
}

func (s *tokenStore) PutAccess(token AccessToken) error {
	return s.setJSON("access:"+token.Token, token, time.Until(token.ExpiresAt))
}

func (s *tokenStore) GetAccess(token string) (AccessToken, bool, error) {
	var record AccessToken
	ok, err := s.getJSON("access:"+token, &record)
	return record, ok, err
}

func (s *tokenStore) DeleteAccess(token string) error {
	return s.client.Del(context.Background(), s.prefix+"access:"+token).Err()
}

func (s *tokenStore) PutRefresh(token RefreshToken) error {
	return s.setJSON("refresh:"+token.Token, token, time.Until(token.ExpiresAt))
}

func (s *tokenStore) GetRefresh(token string) (RefreshToken, bool, error) {
	var record RefreshToken
	ok, err := s.getJSON("refresh:"+token, &record)
	return record, ok, err
}

func (s *tokenStore) DeleteRefresh(token string) error {
	return s.client.Del(context.Background(), s.prefix+"refresh:"+token).Err()
}

// Put 记录一个登录会话。
// access_token 是否有效最终依赖这里保存的会话状态，因此这里是撤销与轮换的核心状态源。
func (s *tokenStore) Put(session Session) error {
	return s.setJSON("session:"+session.SessionID, session, time.Until(session.ExpiresAt))
}

// Get 按 session_id 读取会话状态。
func (s *tokenStore) Get(sessionID string) (Session, bool, error) {
	var record Session
	ok, err := s.getJSON("session:"+sessionID, &record)
	return record, ok, err
}

// Delete 删除会话状态。
// 主动登出或会话彻底失效后，删除该记录即可使整个 access/refresh 会话链路失效。
func (s *tokenStore) Delete(sessionID string) error {
	return s.client.Del(context.Background(), s.prefix+"session:"+sessionID).Err()
}

func (s *tokenStore) setJSON(key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), s.prefix+key, payload, ttl).Err()
}

func (s *tokenStore) getJSON(key string, target any) (bool, error) {
	raw, err := s.client.Get(context.Background(), s.prefix+key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), target)
}

// HashPassword 使用 argon2id 对平台密码做安全哈希。
// 这是平台账号密码，不是校园密码，因此必须使用慢哈希，而不是简单 sha256。
func HashPassword(password string) (string, error) {
	return hashPasswordArgon2ID(password)
}

func hashPasswordArgon2ID(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	const (
		memory      uint32 = 64 * 1024
		iterations  uint32 = 3
		parallelism uint8  = 2
		keyLength   uint32 = 32
	)

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf(
		"argon2id$%d$%d$%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword 同时兼容新的 argon2id 和历史 sha256 格式。
// 历史 sha256 只用于平滑迁移，后续用户一旦成功登录，应尽快升级为 argon2id。
func VerifyPassword(password, encoded string) bool {
	if strings.HasPrefix(encoded, "argon2id$") {
		return verifyArgon2IDPassword(password, encoded)
	}
	return verifyLegacySHA256Password(password, encoded)
}

// PasswordNeedsRehash 用于识别历史弱哈希，登录成功后可以平滑升级。
func PasswordNeedsRehash(encoded string) bool {
	return !strings.HasPrefix(encoded, "argon2id$")
}

func verifyArgon2IDPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		return false
	}

	memory, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return false
	}
	iterations, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return false
	}
	parallelism, err := strconv.ParseUint(parts[3], 10, 8)
	if err != nil {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	hash := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(hash, expected) == 1
}

func verifyLegacySHA256Password(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(parts[2])) == 1
}

func NewOpaqueToken(prefix string) (string, error) {
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(random), nil
}

func ParseUserIDKey(value int64) string {
	return strconv.FormatInt(value, 10)
}
