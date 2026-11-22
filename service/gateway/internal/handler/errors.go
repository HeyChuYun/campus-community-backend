package handler

import (
	"errors"
	"net/http"

	"hfjj/backend/pkg/constants"
)

var (
	// 认证相关错误
	ErrMissingToken       = NewAPIError(constants.CodeUnauthorized, "缺少访问令牌")
	ErrUnauthorized       = NewAPIError(constants.CodeUnauthorized, "访问令牌无效或已过期")
	ErrPermissionDenied   = NewAPIError(constants.CodePermissionDenied, "权限不足")
	ErrAccountUnavailable = NewAPIError(constants.CodePermissionDenied, "账号不可用")

	// 参数相关错误
	ErrInvalidArgument = NewAPIError(constants.CodeInvalidArgument, "请求参数错误")
	ErrInvalidJSON     = NewAPIError(constants.CodeInvalidArgument, "请求JSON格式错误")
	ErrMissingParam    = NewAPIError(constants.CodeInvalidArgument, "缺少必需参数")

	// 资源相关错误
	ErrNotFound = NewAPIError(constants.CodeNotFound, "资源不存在")

	// 系统相关错误
	ErrRPCFailed       = NewAPIError(constants.CodeRPCError, "服务暂时不可用，请稍后重试")
	ErrInternalError   = NewAPIError(constants.CodeInternalError, "系统内部错误")
	ErrInvalidResponse = NewAPIError(constants.CodeUpstreamInvalidResp, "上游服务响应异常")
)

// APIError 统一的API错误
type APIError struct {
	Code    int32
	Message string
	Details map[string]interface{}
}

func NewAPIError(code int32, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

func (e *APIError) Error() string {
	return e.Message
}

func (e *APIError) WithDetails(details map[string]interface{}) *APIError {
	e.Details = details
	return e
}

func (e *APIError) WithDetail(key string, value interface{}) *APIError {
	e.Details[key] = value
	return e
}

// WriteError 统一的错误响应
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Data:    apiErr.Details,
		})
		return
	}

	// 未知错误
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    constants.CodeInternalError,
		Message: "系统内部错误",
	})
}

// WriteSuccess 统一的成功响应
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    constants.CodeOK,
		Message: "ok",
		Data:    data,
	})
}

// WritePassportError 写入passport服务错误
func WritePassportError(w http.ResponseWriter, code int32, message string) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    code,
		Message: message,
	})
}
