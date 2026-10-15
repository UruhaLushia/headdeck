package policy

import "testing"

func TestRoleLadder(t *testing.T) {
	has := func(role Role, perm Permission) bool {
		for _, p := range PermissionsFor(role) {
			if p == perm {
				return true
			}
		}
		return false
	}
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RolePending, PermNodesRead, false},
		{RolePending, PermSystemRead, false},
		{RolePending, PermAccountsRead, false},
		{RolePending, PermAccountsWrite, false},
		{RoleMember, PermNodesRead, true},
		{RoleMember, PermNodesWrite, false},
		{RoleMember, PermKeysRead, true},
		{RoleMember, PermNodesManage, true},
		{RoleMember, PermKeysWrite, true},
		{RoleMember, PermSettingsRead, false},
		{RoleMember, PermAccountsWrite, false},
		{RoleMember, PermAccountsRead, false},
		{RoleMember, PermServiceManage, false},
		{RoleAdmin, PermNodesDelete, true},
		{RoleAdmin, PermAccountsWrite, true},
		{RoleAdmin, PermAccountsRead, true},
		{RoleOwner, PermAccountsWrite, true},
		{RoleOwner, PermAccountsRead, true},
		{RoleOwner, PermServiceManage, true},
		{RoleAdmin, PermServiceManage, true},
	}
	for _, tc := range cases {
		if got := has(tc.role, tc.perm); got != tc.want {
			t.Errorf("%s/%s = %v, want %v", tc.role, tc.perm, got, tc.want)
		}
	}
}
