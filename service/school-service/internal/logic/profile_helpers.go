package logic

import (
	"strconv"
	"strings"
	"time"

	"hfjj/backend/proto/schoolpb"
	"hfjj/backend/school-service/internal/types"
)

// extractStudentNameFromProfile 从个人信息解析结果里提取姓名。
func extractStudentNameFromProfile(profile *types.StudentProfilePayload, fallback ...string) string {
	if profile != nil {
		for key, value := range profile.Fields {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "姓名" || strings.Contains(normalizedKey, "姓名") {
				if strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	for _, item := range fallback {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

// extractStudentStatusFromProfile 从个人信息解析结果里提取学籍状态。
func extractStudentStatusFromProfile(profile *types.StudentProfilePayload) int32 {
	if profile != nil {
		for key, value := range profile.Fields {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "学籍状态" || strings.Contains(normalizedKey, "学籍") {
				return inferStudentStatus(strings.TrimSpace(value))
			}
		}
	}
	return int32(schoolpb.StudentStatus_STUDENT_STATUS_UNKNOWN)
}

// extractAdmissionAtFromProfile 从个人信息解析结果里提取入学时间。
func extractAdmissionAtFromProfile(profile *types.StudentProfilePayload) time.Time {
	if profile == nil {
		return time.Time{}
	}
	for key, value := range profile.Fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "入学日期" || normalizedKey == "入学时间" || strings.Contains(normalizedKey, "入学") {
			if parsed := parseSchoolDateText(value); !parsed.IsZero() {
				return parsed
			}
		}
	}
	return time.Time{}
}

// extractGraduationAtFromProfile 从个人信息解析结果里提取毕业时间。
// 如果学校页面没有直接给出毕业时间，则尝试用入学时间 + 学制推导一个预计毕业时间。
func extractGraduationAtFromProfile(profile *types.StudentProfilePayload, admissionAt time.Time) time.Time {
	if profile == nil {
		return time.Time{}
	}
	for key, value := range profile.Fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "毕业日期" ||
			normalizedKey == "预计毕业日期" ||
			normalizedKey == "毕业时间" ||
			strings.Contains(normalizedKey, "毕业") {
			if parsed := parseSchoolDateText(value); !parsed.IsZero() {
				return parsed
			}
		}
	}

	if admissionAt.IsZero() {
		return time.Time{}
	}
	for key, value := range profile.Fields {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "学制" || strings.Contains(normalizedKey, "学制") {
			years := parseSchoolingYears(value)
			if years <= 0 {
				return time.Time{}
			}
			return admissionAt.AddDate(years, 0, 0)
		}
	}
	return time.Time{}
}

// reconcileStudentStatus 用学校返回状态和时间字段共同确定最终学生状态。
func reconcileStudentStatus(current int32, admissionAt, graduationAt, now time.Time) int32 {
	if !graduationAt.IsZero() && !graduationAt.After(now) {
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_GRADUATED)
	}
	if current == int32(schoolpb.StudentStatus_STUDENT_STATUS_SUSPENDED) {
		return current
	}
	if !admissionAt.IsZero() && !admissionAt.After(now) {
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_IN_SCHOOL)
	}
	if current != int32(schoolpb.StudentStatus_STUDENT_STATUS_UNKNOWN) {
		return current
	}
	return int32(schoolpb.StudentStatus_STUDENT_STATUS_UNKNOWN)
}

// inferStudentStatus 根据学籍状态文本推断平台内部学籍枚举。
func inferStudentStatus(value string) int32 {
	statusText := strings.TrimSpace(value)
	switch {
	case statusText == "":
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_UNKNOWN)
	case strings.Contains(statusText, "在读"), strings.Contains(statusText, "在校"):
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_IN_SCHOOL)
	case strings.Contains(statusText, "毕业"):
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_GRADUATED)
	case strings.Contains(statusText, "休学"), strings.Contains(statusText, "停学"), strings.Contains(statusText, "暂停"):
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_SUSPENDED)
	default:
		return int32(schoolpb.StudentStatus_STUDENT_STATUS_UNKNOWN)
	}
}

// parseSchoolDateText 兼容学校页面里常见的日期格式。
func parseSchoolDateText(value string) time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return time.Time{}
	}

	layouts := []string{
		"20060102",
		"2006-01-02",
		"2006/01/02",
		"2006.01.02",
		"2006-1-2",
		"2006/1/2",
		"2006.1.2",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// parseSchoolingYears 从学校页面的学制字段里提取年限。
func parseSchoolingYears(value string) int {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0
	}

	digits := make([]rune, 0, len(text))
	for _, item := range text {
		if item >= '0' && item <= '9' {
			digits = append(digits, item)
		}
	}
	if len(digits) == 0 {
		return 0
	}

	years, err := strconv.Atoi(string(digits))
	if err != nil || years <= 0 || years > 10 {
		return 0
	}
	return years
}
