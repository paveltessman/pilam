package views

import (
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

// RoleLabel is the one place a role becomes a word the user reads.
//
// It lives here because two screens read it: the users screen, and the audit
// trail that every card carries.
func RoleLabel(role auth.Role) string {
	if role == auth.RootRole {
		return labels.UsersRoleRoot
	}
	return labels.UsersRoleMember
}
