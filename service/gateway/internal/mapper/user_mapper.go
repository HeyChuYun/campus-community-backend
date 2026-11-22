package mapper

import (
	"time"

	"hfjj/backend/proto/passportpb"
)

// UserProfile 统一的用户信息结构
type UserProfile struct {
	UserID        int64
	Account       string
	Nickname      string
	AvatarURL     string
	CoverImageURL string
	Bio           string
	AccountStatus int32
	CreatedAt     time.Time
}

// UserToMap 将用户信息转换为map
func UserToMap(user *UserProfile) map[string]any {
	if user == nil {
		return nil
	}
	return map[string]any{
		"user_id":         user.UserID,
		"account":         user.Account,
		"nickname":        user.Nickname,
		"avatar_url":      user.AvatarURL,
		"cover_image_url": user.CoverImageURL,
		"account_status":  user.AccountStatus,
		"bio":             user.Bio,
		"created_at":      formatTime(user.CreatedAt),
	}
}

// UserProfileToMap 将protobuf用户信息转换为map
func UserProfileToMap(user *passportpb.UserProfile) map[string]any {
	if user == nil {
		return nil
	}
	return map[string]any{
		"user_id":         user.GetUserId(),
		"account":         user.GetAccount(),
		"nickname":        user.GetNickname(),
		"avatar_url":      user.GetAvatarUrl(),
		"cover_image_url": user.GetCoverImageUrl(),
		"account_status":  int32(user.GetAccountStatus()),
		"bio":             user.GetBio(),
		"created_at":      user.GetCreatedAt(),
	}
}

// ToUserProfile 将protobuf用户信息转换为统一结构
func ToUserProfile(user *passportpb.UserProfile) *UserProfile {
	if user == nil {
		return nil
	}

	createdAt, _ := time.Parse(time.RFC3339, user.GetCreatedAt())

	return &UserProfile{
		UserID:        user.GetUserId(),
		Account:       user.GetAccount(),
		Nickname:      user.GetNickname(),
		AvatarURL:     user.GetAvatarUrl(),
		CoverImageURL: user.GetCoverImageUrl(),
		Bio:           user.GetBio(),
		AccountStatus: int32(user.GetAccountStatus()),
		CreatedAt:     createdAt,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}
