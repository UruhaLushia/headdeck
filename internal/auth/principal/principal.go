package principal

import (
	"fmt"
	"sort"

	"github.com/UruhaLushia/headdeck/internal/auth/policy"
)

type Kind string

const (
	PermDNSRead       = policy.PermDNSRead
	PermDNSWrite      = policy.PermDNSWrite
	KindUID      Kind = "uid"

	KindSID Kind = "sid"

	KindWebSession Kind = "web_session"
)

type Role = policy.Role

type Permission = policy.Permission

const (
	RolePending = policy.RolePending
	RoleMember  = policy.RoleMember
	RoleAdmin   = policy.RoleAdmin
	RoleOwner   = policy.RoleOwner
)

const (
	PermSettingsRead   = policy.PermSettingsRead
	PermPolicyRead     = policy.PermPolicyRead
	PermPolicyWrite    = policy.PermPolicyWrite
	PermSettingsWrite  = policy.PermSettingsWrite
	PermSystemRead     = policy.PermSystemRead
	PermNodesRead      = policy.PermNodesRead
	PermNodesWrite     = policy.PermNodesWrite
	PermNodesManage    = policy.PermNodesManage
	PermNodesDelete    = policy.PermNodesDelete
	PermUsersRead      = policy.PermUsersRead
	PermUsersWrite     = policy.PermUsersWrite
	PermAccountsRead   = policy.PermAccountsRead
	PermAccountsWrite  = policy.PermAccountsWrite
	PermKeysRead       = policy.PermKeysRead
	PermKeysWrite      = policy.PermKeysWrite
	PermRoutesRead     = policy.PermRoutesRead
	PermRoutesWrite    = policy.PermRoutesWrite
	PermServiceManage  = policy.PermServiceManage
	PermOperationsRead = policy.PermOperationsRead
	PermAuditRead      = policy.PermAuditRead
)

func ParseRole(raw string) (Role, error) { return policy.ParseRole(raw) }

func PermissionsFor(role Role) []Permission { return policy.PermissionsFor(role) }

func FeaturesFor(perms []Permission) []string { return policy.FeaturesFor(perms) }

type Principal struct {
	Kind Kind
	ID   string
	UID  *uint32
	GID  *uint32
	// PIDs are recycled; use only for audit.
	PID             *int32
	Role            Role
	Label           string
	HeadscaleUserID string

	perms map[Permission]bool
}

func New(kind Kind, id string, role Role) *Principal {
	p := &Principal{Kind: kind, ID: id, Role: role}
	p.setPerms(PermissionsFor(role))
	return p
}

func (p *Principal) setPerms(perms []Permission) {
	p.perms = make(map[Permission]bool, len(perms))
	for _, perm := range perms {
		p.perms[perm] = true
	}
}

func (p *Principal) WithUID(uid uint32) *Principal { p.UID = &uid; return p }

func (p *Principal) WithGID(gid uint32) *Principal { p.GID = &gid; return p }

func (p *Principal) WithPID(pid int32) *Principal { p.PID = &pid; return p }

func (p *Principal) WithLabel(label string) *Principal { p.Label = label; return p }

func (p *Principal) Has(perm Permission) bool {
	return p.perms[perm] && (p.Role != RoleMember || p.HeadscaleUserID != "")
}

func (p *Principal) IsAdmin() bool { return p.Role == RoleAdmin || p.Role == RoleOwner }

func (p *Principal) PermissionList() []Permission {
	out := make([]Permission, 0, len(p.perms))
	for perm := range p.perms {
		if p.Has(perm) {
			out = append(out, perm)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (p *Principal) String() string {
	if p == nil {
		return "anonymous"
	}
	origin := string(p.Kind) + ":" + p.ID
	return fmt.Sprintf("%s role=%s", origin, p.Role)
}
