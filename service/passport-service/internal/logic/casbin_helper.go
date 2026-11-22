package logic

import (
	"fmt"
	"strings"
)

// splitPermissionCode 把权限点编码拆成 Casbin 所需的 obj 和 act。
// 例如 content.post.create -> obj=content.post, act=create。
func splitPermissionCode(permissionCode string) (string, string, error) {
	permissionCode = strings.TrimSpace(permissionCode)
	if permissionCode == "" {
		return "", "", fmt.Errorf("permission_code cannot be empty")
	}

	lastDot := strings.LastIndex(permissionCode, ".")
	if lastDot <= 0 || lastDot >= len(permissionCode)-1 {
		return "", "", fmt.Errorf("invalid permission code: %s", permissionCode)
	}
	return permissionCode[:lastDot], permissionCode[lastDot+1:], nil
}
