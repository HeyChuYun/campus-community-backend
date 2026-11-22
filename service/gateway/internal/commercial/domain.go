package commercial

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/config"
	"hfjj/backend/proto/contentpb"
)

const (
	PlanPremium    = "Premium"
	PlanPremiumPro = "PremiumPro"

	BillingCycleMonth = "month"
	BillingCycleYear  = "year"

	PayChannelAlipay = "alipay"
	PayChannelWechat = "wechat"

	OrderStatusPending int16 = 0
	OrderStatusPaid    int16 = 1

	AdReviewPendingPayment int16 = 0
	AdReviewPendingReview  int16 = 1
	AdReviewApproved       int16 = 2
	AdReviewRejected       int16 = 3

	BizTypeMembership = "membership"
	BizTypeAd         = "ad"

	AdminMembershipActionGrant  = "grant"
	AdminMembershipActionExtend = "extend"
	AdminMembershipActionRevoke = "revoke"

	DurationUnitDay   = "day"
	DurationUnitMonth = "month"
	DurationUnitYear  = "year"
)

type Options struct {
	Ad                AdOptions
	Membership        MembershipOptions
	CandidateTTL      time.Duration
	PayChannels       map[string]string
	PlanAliases       map[string]string
	BillingCycles     map[string]string
	DurationUnits     map[string]string
	MediaPriorityRule map[int]int
}

type AdOptions struct {
	AmountCents         int
	DurationDays        int
	FirstAfterPosts     int
	NextMinPosts        int
	NextMaxPosts        int
	MaxAdsPerPage       int
	MaxAdsPer50Posts    int
	SameCooldownHours   int
	LightReductionAfter int
}

type MembershipOptions struct {
	Policies map[string]MembershipPolicy
}

type MembershipPolicy struct {
	PlanCode              string `json:"plan_code,omitempty"`
	DailyPostBonus        int64  `json:"daily_post_bonus"`
	ImageCountBonus       int64  `json:"image_count_bonus"`
	SingleImageBytesBonus int64  `json:"single_image_bytes_bonus"`
	VideoBytesBonus       int64  `json:"video_bytes_bonus"`
	AdPolicy              string `json:"ad_policy"`
	MediaQueuePriority    int    `json:"media_queue_priority"`
}

type AdCandidate struct {
	OrderNo          string
	AdvertiserUserID int64
	FeedType         string
	StartsAt         time.Time
	EndsAt           time.Time
	Post             *contentpb.PostItem
}

type FeedInsertionPolicy struct {
	FirstAdAfterOrganicPosts int `json:"first_ad_after_organic_posts"`
	NextAdMinOrganicPosts    int `json:"next_ad_min_organic_posts"`
	NextAdMaxOrganicPosts    int `json:"next_ad_max_organic_posts"`
	MaxAdsPerPage            int `json:"max_ads_per_page"`
	MaxAdsPer50Posts         int `json:"max_ads_per_50_posts"`
	SameAdCooldownHours      int `json:"same_ad_cooldown_hours"`
}

func NewOptions(cfg config.CommercialConfig) Options {
	ad := cfg.Advertisement
	opts := Options{
		Ad: AdOptions{
			AmountCents:         positiveInt(ad.AmountCents, 30000),
			DurationDays:        positiveInt(ad.DurationDays, 30),
			FirstAfterPosts:     positiveInt(ad.FirstAdAfterPosts, 7),
			NextMinPosts:        positiveInt(ad.NextAdMinPosts, 10),
			NextMaxPosts:        positiveInt(ad.NextAdMaxPosts, 14),
			MaxAdsPerPage:       positiveInt(ad.MaxAdsPerPage, 1),
			MaxAdsPer50Posts:    positiveInt(ad.MaxAdsPer50Posts, 3),
			SameCooldownHours:   positiveInt(ad.SameAdCooldownHours, 6),
			LightReductionAfter: positiveInt(ad.LightReductionAfter, 12),
		},
		CandidateTTL: positiveDuration(time.Duration(ad.CandidateCacheSecond)*time.Second, 30*time.Second),
		PayChannels: map[string]string{
			PayChannelAlipay: PayChannelAlipay,
			PayChannelWechat: PayChannelWechat,
		},
		PlanAliases: map[string]string{
			"premium":     PlanPremium,
			"premiumpro":  PlanPremiumPro,
			"premium_pro": PlanPremiumPro,
		},
		BillingCycles: map[string]string{
			BillingCycleMonth: BillingCycleMonth,
			BillingCycleYear:  BillingCycleYear,
		},
		DurationUnits: map[string]string{
			DurationUnitDay:   DurationUnitDay,
			DurationUnitMonth: DurationUnitMonth,
			DurationUnitYear:  DurationUnitYear,
		},
		MediaPriorityRule: map[int]int{
			10: 10,
		},
	}
	opts.Membership.Policies = defaultMembershipPolicies()
	for _, item := range cfg.Membership.Policies {
		code := strings.TrimSpace(item.PlanCode)
		if code == "" {
			continue
		}
		opts.Membership.Policies[code] = MembershipPolicy{
			PlanCode:              code,
			DailyPostBonus:        item.DailyPostBonus,
			ImageCountBonus:       item.ImageCountBonus,
			SingleImageBytesBonus: item.SingleImageBytesBonus,
			VideoBytesBonus:       item.VideoBytesBonus,
			AdPolicy:              strings.TrimSpace(item.AdPolicy),
			MediaQueuePriority:    item.MediaQueuePriority,
		}
	}
	return opts
}

func defaultMembershipPolicies() map[string]MembershipPolicy {
	return map[string]MembershipPolicy{
		PlanPremium: {
			PlanCode:              PlanPremium,
			DailyPostBonus:        3,
			ImageCountBonus:       3,
			SingleImageBytesBonus: 20 << 20,
			VideoBytesBonus:       40 << 20,
			AdPolicy:              "light_reduction",
			MediaQueuePriority:    0,
		},
		PlanPremiumPro: {
			PlanCode:              PlanPremiumPro,
			DailyPostBonus:        12,
			ImageCountBonus:       12,
			SingleImageBytesBonus: 60 << 20,
			VideoBytesBonus:       200 << 20,
			AdPolicy:              "no_feed_ads",
			MediaQueuePriority:    10,
		},
	}
}

func (p MembershipPolicy) AllowsFeedAds() bool {
	return p.AdPolicy != "no_feed_ads"
}

func (p MembershipPolicy) LightAdReduction() bool {
	return p.AdPolicy == "light_reduction"
}

func (o Options) InsertionPolicy() FeedInsertionPolicy {
	return FeedInsertionPolicy{
		FirstAdAfterOrganicPosts: o.Ad.FirstAfterPosts,
		NextAdMinOrganicPosts:    o.Ad.NextMinPosts,
		NextAdMaxOrganicPosts:    o.Ad.NextMaxPosts,
		MaxAdsPerPage:            o.Ad.MaxAdsPerPage,
		MaxAdsPer50Posts:         o.Ad.MaxAdsPer50Posts,
		SameAdCooldownHours:      o.Ad.SameCooldownHours,
	}
}

func (o Options) NormalizePayChannel(channel string) (string, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if normalized, ok := o.PayChannels[channel]; ok {
		return normalized, nil
	}
	return "", ErrUnsupportedProvider
}

func (o Options) NormalizePlanCode(planCode string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(planCode))
	if normalized, ok := o.PlanAliases[key]; ok {
		return normalized, nil
	}
	return "", NewError("INVALID_PLAN", "plan_code 必须是 Premium 或 PremiumPro", 400)
}

func (o Options) NormalizeBillingCycle(cycle string) (string, error) {
	cycle = strings.ToLower(strings.TrimSpace(cycle))
	if normalized, ok := o.BillingCycles[cycle]; ok {
		return normalized, nil
	}
	return "", NewError("INVALID_BILLING_CYCLE", "billing_cycle 必须是 month 或 year", 400)
}

func (o Options) NormalizeDurationUnit(unit string) (string, error) {
	unit = strings.ToLower(strings.TrimSpace(unit))
	if normalized, ok := o.DurationUnits[unit]; ok {
		return normalized, nil
	}
	return "", NewError("INVALID_DURATION_UNIT", "duration_unit 蹇呴』鏄?day銆乵onth 鎴?year", 400)
}

func (o Options) ManagedMembershipRoleCodes() []string {
	roleCodes := make([]string, 0, len(o.Membership.Policies))
	for roleCode := range o.Membership.Policies {
		roleCodes = append(roleCodes, roleCode)
	}
	return roleCodes
}

func AddMembershipDuration(start time.Time, unit string, value int) (time.Time, error) {
	if value <= 0 {
		return time.Time{}, ErrInvalidDuration
	}
	switch unit {
	case DurationUnitDay:
		return start.AddDate(0, 0, value), nil
	case DurationUnitMonth:
		return start.AddDate(0, value, 0), nil
	case DurationUnitYear:
		return start.AddDate(value, 0, 0), nil
	default:
		return time.Time{}, NewError("INVALID_DURATION_UNIT", "duration_unit 蹇呴』鏄?day銆乵onth 鎴?year", 400)
	}
}

func NormalizeFeedType(feedType string) string {
	feedType = strings.TrimSpace(feedType)
	if feedType == "" {
		return "discover"
	}
	if len(feedType) > 32 {
		return feedType[:32]
	}
	return feedType
}

func BizTypeByOrderNo(orderNo string) string {
	switch {
	case strings.HasPrefix(orderNo, "MB"):
		return BizTypeMembership
	case strings.HasPrefix(orderNo, "AD"):
		return BizTypeAd
	default:
		return ""
	}
}

func NewOrderNo(prefix string) string {
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s%s%s", prefix, time.Now().Format("20060102150405"), strings.ToUpper(hex.EncodeToString(randomBytes)))
}

func positiveInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
