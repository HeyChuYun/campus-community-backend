package logic

import (
	"regexp"
	"strings"
)

var (
	// 危险的HTML标签和属性
	dangerousTagPattern = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|applet|meta|link|style|base|form)[^>]*>`)
	eventHandlerPattern = regexp.MustCompile(`(?i)\s+on\w+\s*=`)
	dangerousProtocol   = regexp.MustCompile(`(?i)(javascript|data|vbscript):`)
)

// SanitizeContent 使用白名单策略清理用户输入内容
func SanitizeContent(content string) string {
	if content == "" {
		return ""
	}

	// 移除危险标签
	content = dangerousTagPattern.ReplaceAllString(content, "")

	// 移除事件处理器
	content = eventHandlerPattern.ReplaceAllString(content, " ")

	// 移除危险协议
	content = dangerousProtocol.ReplaceAllString(content, "")

	return strings.TrimSpace(content)
}

// ValidateContent 验证内容是否包含危险模式
func ValidateContent(content string) bool {
	if content == "" {
		return true
	}

	// 检查危险标签
	if dangerousTagPattern.MatchString(content) {
		return false
	}

	// 检查事件处理器
	if eventHandlerPattern.MatchString(content) {
		return false
	}

	// 检查危险协议
	if dangerousProtocol.MatchString(content) {
		return false
	}

	return true
}

// EscapeSQLLikePattern 转义SQL LIKE/ILIKE模式中的通配符
func EscapeSQLLikePattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "\\", "\\\\")
	pattern = strings.ReplaceAll(pattern, "%", "\\%")
	pattern = strings.ReplaceAll(pattern, "_", "\\_")
	return pattern
}
