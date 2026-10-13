package policy

import (
	"fmt"
	"sort"
	"strings"
)

type Role string

const (
	RolePending Role = "pending"
	RoleMember  Role = "member"
	RoleAdmin   Role = "admin"
	RoleOwner   Role = "owner"
)

var Roles = []Role{RolePending, RoleMember, RoleAdmin, RoleOwner}

func ParseRole(raw string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(raw)))
	for _, known := range Roles {
		if role == known {
			return role, nil
		}
	}
	return "", fmt.Errorf("unknown role %q (known roles: %s)", raw, RoleNames())
}

func RoleNames() string {
	names := make([]string, 0, len(Roles))
	for _, r := range Roles {
		names = append(names, string(r))
	}
	return strings.Join(names, ", ")
}

type Permission string

const (
	PermSettingsRead   Permission = "settings.read"
	PermDNSRead        Permission = "dns.read"
	PermDNSWrite       Permission = "dns.write"
	PermPolicyRead     Permission = "policy.read"
	PermPolicyWrite    Permission = "policy.write"
	PermSettingsWrite  Permission = "settings.write"
	PermSystemRead     Permission = "system.read"
	PermNodesRead      Permission = "nodes.read"
	PermNodesWrite     Permission = "nodes.write"
	PermNodesManage    Permission = "nodes.manage"
	PermNodesDelete    Permission = "nodes.delete"
	PermUsersRead      Permission = "users.read"
	PermUsersWrite     Permission = "users.write"
	PermAccountsRead   Permission = "accounts.read"
	PermAccountsWrite  Permission = "accounts.write"
	PermKeysRead       Permission = "keys.read"
	PermKeysWrite      Permission = "keys.write"
	PermRoutesRead     Permission = "routes.read"
	PermRoutesWrite    Permission = "routes.write"
	PermServiceManage  Permission = "service.manage"
	PermOperationsRead Permission = "operations.read"
	PermAuditRead      Permission = "audit.read"
)

func PermissionsFor(role Role) []Permission {
	switch role {
	case RoleOwner:
		return append(PermissionsFor(RoleAdmin), PermAccountsRead, PermAccountsWrite)
	case RoleAdmin:
		return []Permission{
			PermDNSRead, PermDNSWrite, PermPolicyRead, PermPolicyWrite,
			PermSystemRead, PermNodesRead, PermNodesWrite, PermNodesManage, PermNodesDelete,
			PermUsersRead, PermUsersWrite, PermKeysRead, PermKeysWrite, PermRoutesRead, PermRoutesWrite,
			PermServiceManage, PermOperationsRead, PermAuditRead, PermSettingsRead, PermSettingsWrite,
		}
	case RoleMember:
		return []Permission{PermNodesRead, PermNodesManage, PermUsersRead, PermKeysRead, PermKeysWrite, PermRoutesRead}
	default:
		return nil
	}
}

// Advertise capabilities allowed by the current principal.
func FeaturesFor(perms []Permission) []string {
	seen := map[string]bool{}
	for _, p := range perms {
		switch p {
		case PermDNSRead, PermDNSWrite, PermPolicyRead, PermPolicyWrite:
			seen[string(p)] = true
		case PermSettingsRead:
			seen["settings.read"] = true
			seen["headscale.config.read"] = true
		case PermSettingsWrite:
			seen["settings.write"] = true
			seen["headscale.config.write"] = true
		case PermSystemRead:
			seen["system.read"] = true
		case PermNodesRead:
			seen["nodes.read"] = true
		case PermNodesWrite:
			seen["nodes.write"] = true
		case PermNodesManage:
			seen["nodes.manage"] = true
		case PermNodesDelete:
			seen["nodes.delete"] = true
		case PermUsersRead:
			seen["users.read"] = true
		case PermUsersWrite:
			seen["users.write"] = true
		case PermAccountsRead:
			seen["accounts.read"] = true
		case PermAccountsWrite:
			seen["accounts.write"] = true
		case PermKeysRead:
			seen["keys.read"] = true
		case PermKeysWrite:
			seen["keys.write"] = true
		case PermRoutesRead:
			seen["routes.read"] = true
		case PermRoutesWrite:
			seen["routes.write"] = true
		case PermServiceManage:
			for _, action := range []string{"start", "stop", "restart", "reload"} {
				seen["headscale."+action] = true
			}
		case PermOperationsRead:
			seen["system.operations"] = true
		case PermAuditRead:
			seen["audit.read"] = true
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
