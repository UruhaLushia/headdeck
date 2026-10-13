package tui

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type resourceAction struct{ kind, label, help, value, id, target, confirm string }

func (m *Model) can(feature string) bool {
	return m.hello != nil && slices.Contains(m.hello.Features, feature)
}
func (m *Model) dialogOpen() bool {
	return m.objectForm != nil || m.editKey != "" || m.menu != nil || m.searching || m.settingPopup != nil || m.confirmation != nil
}

func (m *Model) openManagement() {
	m.menu = nil
	m.menuCursor = 0
	m.focus = focusContent
	m.err = nil
	m.notice = ""
	if m.page == pageKeys {
		m.keyMenu()
		return
	}
	if m.page == pageStatus {
		m.openServiceMenu()
		return
	}
	if m.page == pageSettings {
		return
	}
	if m.page == pageUsers {
		if !m.can("users.write") {
			return
		}
		users := m.visibleUsers()
		if m.cursor >= 0 && m.cursor < len(users) {
			u := users[m.cursor]
			if u.Provider != "oidc" {
				m.menu = append(m.menu, resourceAction{kind: "user.rename", label: "Rename user", id: u.ID, target: u.Name, value: u.Name, help: "The user ID and account binding will stay unchanged."})
			}
			m.menu = append(m.menu, resourceAction{kind: "user.delete", label: "Delete user", id: u.ID, target: u.Name, confirm: u.Name, help: "Linked accounts and owned devices must be detached first."})
		}
		return
	}
	n, ok := m.selectedNode()
	if m.page == pageNodeDetail && m.detail != nil {
		n, ok = *m.detail, true
	}
	if !ok {
		return
	}
	name := n.GivenName
	if name == "" {
		name = n.Name
	}
	add := func(kind, label, help, value, confirm string) {
		m.menu = append(m.menu, resourceAction{kind: kind, label: label, help: help, value: value, confirm: confirm, id: n.ID, target: name})
	}
	if m.can("nodes.write") {
		add("node.rename", "Rename device", "Enter a lowercase device name.", name, "")
		add("node.tags", "Edit tags", "Separate tags with commas. An empty value clears all forced tags.", strings.Join(n.ForcedTags, ", "), "")
		if len(m.users) > 0 {
			add("node.owner", "Change owner", "Use ↑/↓ or the mouse wheel to choose a Headscale user.", n.User.ID, "")
		}
		add("node.expire", "Expire device", "The device will need to sign in again.", "", name)
	}
	if m.can("routes.write") {
		add("node.routes", "Approve routes", "Separate CIDR prefixes with commas. An empty value clears all approvals.", strings.Join(n.ApprovedRoutes, ", "), "")
	}
	if m.can("nodes.delete") {
		add("node.delete", "Delete device", "This cannot be undone.", "", name)
	}
}

func (m *Model) managementView() string {
	lines := []string{m.section("Manage", "Choose an operation for the selected resource"), "", dimStyle.Render(m.tr("ACTION"))}
	for i, a := range m.menu {
		line := cell(m.tr(a.label), min(28, m.contentWidth()/2)) + "  " + a.target
		if strings.HasPrefix(a.kind, "service.") {
			line = m.tr(a.label)
		}
		if i == m.menuCursor {
			line = selStyle.Render(cell(line, m.contentWidth()))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) managementKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.menuCursor = max(0, m.menuCursor-1)
	case "down", "j":
		m.menuCursor = min(len(m.menu)-1, m.menuCursor+1)
	case "home":
		m.menuCursor = 0
	case "end":
		m.menuCursor = max(0, len(m.menu)-1)
	case "esc", "left":
		m.menu = nil
	case "enter":
		if m.menuCursor >= 0 && m.menuCursor < len(m.menu) {
			m.beginResource(m.menu[m.menuCursor])
		}
	}
	return m, nil
}

func (m *Model) beginResource(a resourceAction) {
	if a.confirm != "" {
		m.openConfirmation(a)
		return
	}
	m.resource = &a
	m.menu = nil
	m.editKey = "resource:" + a.kind
	m.editValue = a.value
	m.editCursor = len([]rune(a.value))
	m.focus = focusContent
	m.err = nil
	m.notice = ""
	m.openResourceForm(a)
}
