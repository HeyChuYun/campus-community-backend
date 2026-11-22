package userquery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	userSearchLookaheadLimit int64 = 1
	schoolAccountBoundStatus int64 = 1
)

type Service struct {
	db *gorm.DB
}

type User struct {
	ID            int64     `gorm:"column:id"`
	Account       string    `gorm:"column:account"`
	Nickname      string    `gorm:"column:nickname"`
	AvatarURL     string    `gorm:"column:avatar_url"`
	CoverImageURL string    `gorm:"column:cover_image_url"`
	Bio           string    `gorm:"column:bio"`
	AccountStatus int32     `gorm:"column:account_status"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

type SchoolAccount struct {
	UserID      int64  `gorm:"column:user_id"`
	StudentNo   string `gorm:"column:student_no"`
	StudentName string `gorm:"column:student_name"`
}

type LoginRecord struct {
	LoginIP string    `gorm:"column:login_ip"`
	Device  string    `gorm:"column:device"`
	LoginAt time.Time `gorm:"column:login_at"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: requireGormDB(db)}
}

func (s *Service) ByID(ctx context.Context, userID int64) (User, bool, error) {
	if userID <= 0 {
		return User{}, false, nil
	}
	var user User
	err := s.baseUserQuery(ctx).Where("id = ?", userID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Service) ByAccount(ctx context.Context, account string) (User, bool, error) {
	if account == "" {
		return User{}, false, nil
	}
	var user User
	err := s.baseUserQuery(ctx).Where("account = ?", account).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Service) BatchByIDs(ctx context.Context, userIDs []int64) ([]User, error) {
	userIDs = dedupePositiveInt64s(userIDs)
	if len(userIDs) == 0 {
		return []User{}, nil
	}
	var users []User
	err := s.baseUserQuery(ctx).Where("id IN ?", userIDs).Find(&users).Error
	return users, err
}

func (s *Service) Search(ctx context.Context, keyword string, page, pageSize int64) ([]User, bool, error) {
	offset, limit := normalizePage(page, pageSize)
	like := "%" + keyword + "%"
	var users []User
	err := s.baseUserQuery(ctx).
		Where("account ILIKE ? OR nickname ILIKE ?", like, like).
		Order(gorm.Expr(`
case
  when lower(account) = lower(?) then 0
  when lower(nickname) = lower(?) then 1
  when lower(account) like lower(?) || '%' then 2
  when lower(nickname) like lower(?) || '%' then 3
  else 4
end`, keyword, keyword, keyword, keyword)).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit + userSearchLookaheadLimit)).
		Find(&users).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := int64(len(users)) > limit
	if hasMore {
		users = users[:limit]
	}
	return users, hasMore, nil
}

func (s *Service) SchoolAccountByUserID(ctx context.Context, userID int64) (SchoolAccount, bool, error) {
	if userID <= 0 {
		return SchoolAccount{}, false, nil
	}
	var account SchoolAccount
	err := s.db.WithContext(ctx).
		Table("school_accounts").
		Select("user_id, student_no, student_name").
		Where("user_id = ? AND bind_status = ?", userID, schoolAccountBoundStatus).
		Limit(1).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SchoolAccount{}, false, nil
	}
	if err != nil {
		return SchoolAccount{}, false, err
	}
	return account, true, nil
}

func (s *Service) SchoolAccountByStudentNo(ctx context.Context, studentNo string) (SchoolAccount, bool, error) {
	if studentNo == "" {
		return SchoolAccount{}, false, nil
	}
	var account SchoolAccount
	err := s.db.WithContext(ctx).
		Table("school_accounts").
		Select("user_id, student_no, student_name").
		Where("student_no = ? AND bind_status = ?", studentNo, schoolAccountBoundStatus).
		Limit(1).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SchoolAccount{}, false, nil
	}
	if err != nil {
		return SchoolAccount{}, false, err
	}
	return account, true, nil
}

func (s *Service) LoginRecordsByUserID(ctx context.Context, userID, limit int64) ([]LoginRecord, error) {
	if userID <= 0 {
		return []LoginRecord{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var items []LoginRecord
	err := s.db.WithContext(ctx).
		Table("user_login_records").
		Select("login_ip, device, login_at").
		Where("user_id = ?", userID).
		Order("login_at DESC").
		Limit(int(limit)).
		Find(&items).Error
	return items, err
}

func (s *Service) baseUserQuery(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).
		Table("users").
		Select("id, account, nickname, avatar_url, cover_image_url, bio, account_status, created_at").
		Where("deleted_at IS NULL")
}

func normalizePage(page, pageSize int64) (int64, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return (page - 1) * pageSize, pageSize
}

func dedupePositiveInt64s(values []int64) []int64 {
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
		panic("gateway userquery: gorm db is required")
	}
	return db
}
