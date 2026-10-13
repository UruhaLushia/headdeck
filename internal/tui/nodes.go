package tui

import (
	"strings"
)

func (m *Model) nodesView() string {
	nodes := m.visibleNodes()
	intro := m.section("Nodes", m.tr("%d devices · %s", len(nodes), m.filterDescription()))
	if len(nodes) == 0 {
		return intro + "\n\n" + dimStyle.Render(m.tr("No matching nodes. Press / to change the filter."))
	}
	w := m.contentWidth()
	start, end := m.listWindow(len(nodes), 6)
	if len(nodes) > end-start {
		w -= 2
	}
	nameWidth := max(8, w-38)
	header := cell(m.tr("DEVICE"), nameWidth) + "  " + cell(m.tr("OWNER"), 12) + "  " + cell(m.tr("STATE"), 9) + "  " + m.tr("LAST SEEN")
	if w < 65 {
		nameWidth = max(6, w-12)
		header = cell(m.tr("DEVICE"), nameWidth) + "  " + m.tr("STATE")
	}
	lines := []string{intro, "", dimStyle.Render(header)}
	if w >= 92 {
		lines[2] = dimStyle.Render(m.nodeTableHeader(w))
	}
	for i := start; i < end; i++ {
		n := nodes[i]
		name := n.GivenName
		if name == "" {
			name = dashIfEmpty(n.Name)
		}
		state := m.tr("○ offline")
		if n.Online {
			state = m.tr("● online")
		} else if n.Expired {
			state = m.tr("◷ expired")
		}
		line := cell(name, nameWidth) + "  " + cell(n.User.Name, 12) + "  " + cell(state, 9) + "  " + shortTime(n.LastSeen)
		if w < 65 {
			line = cell(name, nameWidth) + "  " + state
		}
		if w >= 92 {
			line = m.nodeTableRow(n, w)
		}
		lines = append(lines, m.selectedRow(line, i))
	}
	lines = append(lines, "", dimStyle.Render(m.tr("%d–%d of %d · ↑/↓ navigate", start+1, end, len(nodes))))
	return strings.Join(lines, "\n")
}

func (m *Model) filterDescription() string {
	if m.searching {
		return m.tr("Filter: %s", m.filter) + "▏"
	}
	if m.filter != "" {
		return m.tr("Filter: %s", m.filter)
	}
	return m.tr("live updates")
}

func (m *Model) detailView() string {
	if m.detail == nil {
		return m.section("Node details", "Loading device…")
	}
	n := m.detail
	rows := m.detailRows()
	m.scrollDetail(0)
	end := min(len(rows), m.detailOffset+max(1, m.height-13))
	return m.section("Node details", m.tr("Device #%s", n.ID)) + "\n\n" + detailRows(m.translatedRows(rows[m.detailOffset:end]), m.contentWidth())
}

func (m *Model) scrollDetail(step int) {
	m.detailOffset = max(0, min(m.detailOffset+step, len(m.detailRows())-max(1, m.height-13)))
}

func (m *Model) detailRows() [][2]string {
	if m.detail == nil {
		return nil
	}
	n := m.detail
	return [][2]string{
		{"Name", dashIfEmpty(n.GivenName)}, {"Hostname", dashIfEmpty(n.Name)},
		{"Owner", dashIfEmpty(n.User.Name)}, {"Addresses", dashIfEmpty(strings.Join(n.IPAddresses, ", "))},
		{"State", m.tr(onOffText(n.Online))}, {"Expired", m.tr(yesNo(n.Expired))},
		{"Last seen", shortTime(n.LastSeen)}, {"Expires", shortTime(n.Expiry)},
		{"Registration", dashIfEmpty(n.RegisterMethod)},
		{"Available routes", dashIfEmpty(strings.Join(n.AvailableRoutes, ", "))},
		{"Approved routes", dashIfEmpty(strings.Join(n.ApprovedRoutes, ", "))},
		{"Tags", dashIfEmpty(strings.Join(n.ValidTags, ", "))},
	}
}

func (m *Model) usersView() string {
	users := m.visibleUsers()
	intro := m.section("Users", m.tr("%d accounts · %s", len(users), m.filterDescription()))
	if len(users) == 0 {
		return intro + "\n\n" + dimStyle.Render(m.tr("No matching users. Press / to change the filter."))
	}
	w := m.contentWidth()
	nameWidth := max(8, w/3)
	header := cell(m.tr("ACCOUNT"), nameWidth) + "  " + cell(m.tr("DISPLAY NAME"), nameWidth) + "  " + m.tr("EMAIL")
	if w < 65 {
		header = m.tr("ACCOUNT")
	}
	lines := []string{intro, "", dimStyle.Render(header)}
	start, end := m.listWindow(len(users), 6)
	for i := start; i < end; i++ {
		u := users[i]
		line := cell(u.Name, nameWidth) + "  " + cell(dashIfEmpty(u.DisplayName), nameWidth) + "  " + dashIfEmpty(u.Email)
		if w < 65 {
			line = u.Name
		}
		lines = append(lines, m.selectedRow(line, i))
	}
	lines = append(lines, "", dimStyle.Render(m.tr("%d–%d of %d · ↑/↓ navigate", start+1, end, len(users))))
	return strings.Join(lines, "\n")
}
