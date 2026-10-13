package tui

import (
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"strings"
)

func (m *Model) visibleNodes() []protocol.Node {
	if m.filter == "" {
		return m.nodes
	}
	needle := strings.ToLower(m.filter)
	out := make([]protocol.Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		haystack := strings.ToLower(strings.Join([]string{
			n.ID, n.GivenName, n.Name, n.User.Name, strings.Join(n.IPAddresses, " "),
		}, " "))
		if strings.Contains(haystack, needle) {
			out = append(out, n)
		}
	}
	return out
}

func (m *Model) selectedNode() (protocol.Node, bool) {
	nodes := m.visibleNodes()
	if m.cursor < 0 || m.cursor >= len(nodes) {
		return protocol.Node{}, false
	}
	return nodes[m.cursor], true
}

func (m *Model) visibleUsers() []protocol.User {
	if m.filter == "" {
		return m.users
	}
	needle := strings.ToLower(m.filter)
	var users []protocol.User
	for _, u := range m.users {
		if strings.Contains(strings.ToLower(u.Name+" "+u.DisplayName+" "+u.Email), needle) {
			users = append(users, u)
		}
	}
	return users
}
