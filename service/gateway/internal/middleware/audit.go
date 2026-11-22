package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AuditLogger struct {
	db *gorm.DB
}

type AuditLog struct {
	UserID       int64
	Action       string
	Resource     string
	ResourceID   int64
	Method       string
	Path         string
	IP           string
	UserAgent    string
	RequestBody  string
	ResponseCode int
	ErrorMessage string
	CreatedAt    time.Time
}

func NewAuditLogger(db *gorm.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

func (al *AuditLogger) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 只记录写操作
			if !isWriteOperation(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// 读取请求体
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// 包装ResponseWriter以捕获状态码
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// 执行请求
			next.ServeHTTP(rw, r)

			// 异步记录审计日志
			go al.logAudit(r, bodyBytes, rw.statusCode)
		})
	}
}

func (al *AuditLogger) logAudit(r *http.Request, bodyBytes []byte, statusCode int) {
	if al.db == nil {
		return
	}

	userID := getUserIDFromContext(r.Context())
	action, resource, resourceID := parseActionFromPath(r.URL.Path, r.Method)

	log := AuditLog{
		UserID:       userID,
		Action:       action,
		Resource:     resource,
		ResourceID:   resourceID,
		Method:       r.Method,
		Path:         r.URL.Path,
		IP:           getClientIP(r),
		UserAgent:    r.UserAgent(),
		RequestBody:  sanitizeRequestBody(bodyBytes),
		ResponseCode: statusCode,
		CreatedAt:    time.Now(),
	}

	if err := al.saveAuditLog(log); err != nil {
		logx.Errorf("Failed to save audit log: %v", err)
	}
}

func (al *AuditLogger) saveAuditLog(log AuditLog) error {
	return al.db.WithContext(context.Background()).Table("audit_logs").Create(map[string]any{
		"user_id":       log.UserID,
		"action":        log.Action,
		"resource":      log.Resource,
		"resource_id":   nullInt64(log.ResourceID),
		"method":        log.Method,
		"path":          log.Path,
		"ip":            log.IP,
		"user_agent":    log.UserAgent,
		"request_body":  log.RequestBody,
		"response_code": log.ResponseCode,
		"error_message": log.ErrorMessage,
		"created_at":    log.CreatedAt,
	}).Error
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func isWriteOperation(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch || method == http.MethodDelete
}

func parseActionFromPath(path, method string) (action, resource string, resourceID int64) {
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) < 3 {
		return method, path, 0
	}

	// 解析资源类型
	if len(parts) >= 4 {
		resource = parts[3]
	}

	// 解析操作类型
	switch {
	case strings.Contains(path, "/delete"):
		action = "delete"
	case strings.Contains(path, "/create") || method == http.MethodPost:
		action = "create"
	case strings.Contains(path, "/update") || method == http.MethodPut:
		action = "update"
	case strings.Contains(path, "/assign"):
		action = "assign"
	default:
		action = method
	}

	return action, resource, 0
}

func sanitizeRequestBody(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}

	// 限制长度
	if len(bodyBytes) > 4096 {
		bodyBytes = bodyBytes[:4096]
	}

	// 解析JSON并移除敏感字段
	var data map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return string(bodyBytes)
	}

	// 移除敏感字段
	sensitiveFields := []string{"password", "school_password", "access_token", "refresh_token"}
	for _, field := range sensitiveFields {
		if _, ok := data[field]; ok {
			data[field] = "***REDACTED***"
		}
	}

	sanitized, _ := json.Marshal(data)
	return string(sanitized)
}

func getUserIDFromContext(ctx context.Context) int64 {
	if userID, ok := ctx.Value("user_id").(int64); ok {
		return userID
	}
	return 0
}

func nullInt64(value int64) interface{} {
	if value == 0 {
		return nil
	}
	return value
}
