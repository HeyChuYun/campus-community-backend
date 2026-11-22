package backoffice

import (
	"strings"

	"hfjj/backend/proto/passportpb"
)

type AccessPolicy struct {
	RoleCodes          map[string]struct{}
	PermissionPrefixes []string
	PermissionCodes    map[string]struct{}
}

func DefaultAccessPolicy() AccessPolicy {
	return AccessPolicy{
		RoleCodes: map[string]struct{}{
			"admin":    {},
			"reviewer": {},
			"auditor":  {},
			"merchant": {},
			"operator": {},
		},
		PermissionPrefixes: []string{
			"admin.",
			"review.",
			"merchant.",
		},
		PermissionCodes: map[string]struct{}{
			"backoffice.access": {},
		},
	}
}

func (p AccessPolicy) Allows(snapshot *passportpb.AuthorizationSnapshot) bool {
	if snapshot == nil {
		return false
	}
	for _, role := range snapshot.GetRoles() {
		if _, ok := p.RoleCodes[strings.ToLower(strings.TrimSpace(role.GetRoleCode()))]; ok {
			return true
		}
	}
	for _, permission := range snapshot.GetPermissions() {
		code := strings.ToLower(strings.TrimSpace(permission.GetPermissionCode()))
		if _, ok := p.PermissionCodes[code]; ok {
			return true
		}
		for _, prefix := range p.PermissionPrefixes {
			if strings.HasPrefix(code, prefix) {
				return true
			}
		}
	}
	return false
}
