package commercial

import (
	"errors"
	"net/http"
)

var (
	ErrInvalidArgument       = NewError("INVALID_ARGUMENT", "请求参数错误", http.StatusBadRequest)
	ErrInvalidDuration       = NewError("INVALID_DURATION", "会员时长参数无效", http.StatusBadRequest)
	ErrDatabaseUnavailable   = NewError("DATABASE_UNAVAILABLE", "数据库不可用", http.StatusInternalServerError)
	ErrPostNotOwned          = NewError("POST_NOT_OWNED", "帖子不存在或不属于当前用户", http.StatusBadRequest)
	ErrOrderNotFound         = NewError("ORDER_NOT_FOUND", "订单不存在", http.StatusNotFound)
	ErrOrderNotPaid          = NewError("ORDER_NOT_PAID", "订单未支付", http.StatusBadRequest)
	ErrOrderNotPendingReview = NewError("ORDER_NOT_PENDING_REVIEW", "订单不是待审核状态", http.StatusBadRequest)
	ErrUnsupportedOrderType  = NewError("UNSUPPORTED_ORDER_TYPE", "不支持的订单类型", http.StatusBadRequest)
	ErrUnsupportedProvider   = NewError("UNSUPPORTED_PROVIDER", "不支持的支付渠道", http.StatusBadRequest)
	ErrMembershipNotFound    = NewError("MEMBERSHIP_NOT_FOUND", "会员记录不存在", http.StatusNotFound)
)

type Error struct {
	Code       string
	Message    string
	HTTPStatus int
}

func NewError(code, message string, httpStatus int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: httpStatus}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func AsError(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
