package logic

import (
	"errors"
	"strings"

	"hfjj/backend/proto/schoolpb"
	"hfjj/backend/school-service/internal/repository"
)

// statusFromError 把内部错误转换成 proto 里的统一状态结构。
func statusFromError(err error) *schoolpb.BaseResp {
	if err == nil {
		return newStatus(schoolpb.SchoolCode_SCHOOL_CODE_OK, "ok")
	}

	var campusErr *repository.CampusError
	if errors.As(err, &campusErr) {
		return newStatus(campusErr.Code, campusErr.Message)
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "unknown error"
	}

	return newStatus(schoolpb.SchoolCode_SCHOOL_CODE_INTERNAL_ERROR, message)
}

// newStatus 用于统一创建响应状态对象。
func newStatus(code schoolpb.SchoolCode, message string) *schoolpb.BaseResp {
	return &schoolpb.BaseResp{
		Code:    code,
		Message: message,
	}
}
