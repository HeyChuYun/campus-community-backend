package reporting

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrNotFound = errors.New("reporting resource not found")

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

const (
	TargetTypeUser = "user"
	TargetTypePost = "post"

	StatusPending             = "pending"
	StatusReviewing           = "reviewing"
	StatusWaitingMoreEvidence = "waiting_more_evidence"
	StatusResolvedValid       = "resolved_valid"
	StatusResolvedInvalid     = "resolved_invalid"
	StatusClosed              = "closed"
)

type Settings struct {
	DailyTargetLimit    int
	SameTargetCooldown  int
	MinReasonLength     int
	MaxReasonLength     int
	MaxEvidenceCount    int
	MaxEvidenceSizeByte int64
}

type StatusPolicy struct {
	DuplicateBlocking map[string]struct{}
	CooldownSkipping  map[string]struct{}
}

func DefaultStatusPolicy() StatusPolicy {
	return StatusPolicy{
		DuplicateBlocking: map[string]struct{}{
			StatusPending:             {},
			StatusReviewing:           {},
			StatusWaitingMoreEvidence: {},
		},
		CooldownSkipping: map[string]struct{}{
			StatusResolvedValid:   {},
			StatusResolvedInvalid: {},
			StatusClosed:          {},
		},
	}
}

func (p StatusPolicy) BlocksDuplicate(status string) bool {
	_, ok := p.DuplicateBlocking[status]
	return ok
}

func (p StatusPolicy) SkipsCooldown(status string) bool {
	_, ok := p.CooldownSkipping[status]
	return ok
}

type Category struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type EvidenceInput struct {
	FileURL  string
	FileSize int64
	Width    int
	Height   int
}

type CreateUserReportInput struct {
	ReporterUserID int64
	TargetUserID   int64
	CategoryCode   string
	ReasonText     string
	Evidences      []EvidenceInput
	TargetSnapshot map[string]any
}

type CreateUserReportResult struct {
	ReportID int64
	ReportNo string
	Status   string
}

type PostAppealContext struct {
	PostID                int64
	AuthorUserID          int64
	ContentText           string
	PostStatus            int32
	LatestViolationNo     string
	LatestViolationAt     *time.Time
	LatestViolationSource string
	LatestReason          string
	LatestCategories      []string
	MuteScope             string
	MuteDurationSeconds   int64
	PostMuteUntil         *time.Time
	HasAppeal             bool
	LatestAppealStatus    string
	LatestAppealAt        *time.Time
	CanSubmitAppeal       bool
	BlockSubmitReason     string
}

type CreatePostAppealInput struct {
	ReporterUserID int64
	TargetUserID   int64
	TargetPostID   int64
	ReasonText     string
	Evidences      []EvidenceInput
	TargetSnapshot map[string]any
}

type CreatePostAppealResult struct {
	ReportID int64
	ReportNo string
	Status   string
}

type AdminListFilter struct {
	Page       int
	PageSize   int
	Status     string
	Keyword    string
	TargetType string
}

type AdminListResult struct {
	Items []AdminReportItem
	Total int64
}

type AdminReportItem struct {
	ReportID           int64
	ReportNo           string
	ReporterUserID     int64
	TargetType         string
	TargetUserID       int64
	TargetPostID       int64
	CategoryCode       string
	ReasonText         string
	Status             string
	LatestDecisionCode string
	LatestDecisionNote string
	AssignedToUserID   int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AdminReportDetail struct {
	Report    AdminReportDetailItem
	Evidences []EvidenceItem
	Actions   []ActionItem
}

type AdminReportDetailItem struct {
	ReportID           int64
	ReportNo           string
	ReporterUserID     int64
	TargetType         string
	TargetUserID       int64
	TargetPostID       int64
	CategoryCode       string
	ReasonText         string
	Status             string
	TargetSnapshot     json.RawMessage
	LatestDecisionCode string
	LatestDecisionNote string
	AssignedToUserID   int64
	ClosedAt           *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type EvidenceItem struct {
	ID        int64
	MediaType string
	FileURL   string
	FileSize  int64
	Width     int
	Height    int
	SortOrder int
	CreatedAt time.Time
}

type ActionItem struct {
	ID             int64
	OperatorUserID int64
	ActionType     string
	FromStatus     string
	ToStatus       string
	Note           string
	Extra          json.RawMessage
	CreatedAt      time.Time
}

type AdminStatusUpdate struct {
	ReportID       int64
	Status         string
	DecisionNote   string
	DecisionCode   string
	AssignedToUID  int64
	OperatorUserID int64
}
