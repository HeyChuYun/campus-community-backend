package backoffice

import "time"

const (
	heartbeatTTL       = 120 * time.Second
	onlineThreshold    = int64(120)
	recentHeartbeatTTL = 30 * 24 * time.Hour
	recentActiveWindow = 7 * 24 * time.Hour

	UserRestrictionScopePostPublish   = "post_publish"
	UserRestrictionScopeDirectMessage = "direct_message"
	UserRestrictionScopeAccountLogin  = "account_login"
)

type HeartbeatInput struct {
	UserID   int64
	Version  string
	Platform string
	Device   string
}

type DashboardStats struct {
	OnlineCount          int64
	TotalUsers           int64
	TodayNewUsers        int64
	VersionDistribution  []DistributionItem
	PlatformDistribution []DistributionItem
	DeviceDistribution   []DistributionItem
}

type DistributionItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type UserListFilter struct {
	Page     int
	PageSize int
	Search   string
}

type UserListResult struct {
	Users []UserItem
	Total int64
}

type UserItem struct {
	ID              int64
	Account         string
	Nickname        string
	AvatarURL       string
	AccountStatus   int
	PostMuteUntil   *time.Time
	DMMuteUntil     *time.Time
	AccountBanUntil *time.Time
	ViolationCount  int64
	CreatedAt       time.Time
	Roles           []string
	RoleNames       []string
}

type UserRestrictionInput struct {
	UserID         int64
	Scope          string
	Reason         string
	Duration       time.Duration
	OperatorUserID int64
}
