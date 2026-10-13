package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var navigationPages = []page{pageStatus, pageNodes, pageUsers, pageKeys, pageDNS, pageHeadscale, pagePolicy, pageSettings, pageAccounts, pageProviders}

func (m *Model) View() string {
	if m.width == 0 {
		return m.tr("Connecting to Headdeck…")
	}
	l := m.layout()
	w, h := l.mainWidth, max(1, m.height-10)
	limit := max(1, w-4)
	if m.statusPanels() {
		h, limit = max(1, m.height-6), w
	}
	lines := strings.Split(m.content(), "\n")
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], limit, "…")
	}
	if len(lines) > h {
		lines = lines[:h]
		lines[h-1] = dimStyle.Render(m.tr("Resize the terminal to show more."))
	}
	style := panelStyle
	if m.focus == focusContent {
		style = style.BorderForeground(accent)
	}
	panel := style.Width(max(1, w-2)).Height(h + 2).Render(strings.Join(lines, "\n"))
	top := m.tabs(w)
	if l.sidebar > 0 {
		top = dimStyle.Render("Headscale  /  ") + titleStyle.Render(m.tr(pageNames[m.page]))
	}
	body := strings.Join([]string{top, panel}, "\n")
	if m.statusPanels() {
		content := strings.Join(lines, "\n")
		if l.sidebar == 0 {
			content = top + "\n" + content
		}
		body = lipgloss.NewStyle().Width(w).Height(max(1, m.height-5)).Render(content)
	}
	if l.sidebar > 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.sidebar(l.sidebar-2, m.height-6), "  ", body)
	}
	view := lipgloss.NewStyle().Padding(1, 2).Render(strings.Join([]string{
		m.header(max(1, m.width-4)), dimStyle.Render(strings.Repeat("─", max(1, m.width-4))), body,
		strings.Repeat(" ", l.sidebar) + m.footer(w),
	}, "\n"))
	if m.objectForm != nil {
		view = m.overlayPopup(view, m.objectFrame().selectionFrame)
	}
	if m.settingPopup != nil {
		return m.overlaySettingPopup(view)
	}
	if m.confirmation != nil {
		return m.overlayPopup(view, m.confirmationFrame())
	}
	return view
}

func (m *Model) header(w int) string {
	brand := titleStyle.Render("HEADDECK") + dimStyle.Render("  /  "+m.tr("Control center"))
	if m.status != nil && w >= 100 {
		brand = titleStyle.Render("HEADDECK") + dimStyle.Render("  "+m.status.Server.Version+"  /  "+m.tr("Control center"))
	}
	if w < 55 {
		brand = titleStyle.Render("HEADDECK")
	}
	state := warnStyle.Render(m.tr("● Connecting"))
	switch m.conn {
	case connReady:
		state = okStyle.Render(m.tr("● Connected"))
	case connError:
		state = errStyle.Render(m.tr("● Disconnected"))
	}
	if m.hello != nil && w >= 55 {
		state += dimStyle.Render("  │  " + m.tr(string(m.hello.Role)))
	}
	if w >= 100 {
		now := time.Now()
		if m.now != nil {
			now = m.now()
		}
		state += dimStyle.Render("  │  " + now.Format("2006-01-02 15:04"))
	}
	brand = ansi.Truncate(brand, max(1, w-lipgloss.Width(state)-2), "…")
	return brand + strings.Repeat(" ", max(1, w-lipgloss.Width(brand)-lipgloss.Width(state))) + state
}

func (m *Model) tabs(w int) string {
	var items []string
	x := m.layout().mainX
	start, end := m.navigationWindow()
	for i := start; i < end; i++ {
		p := navigationPages[i]
		label := m.tabLabel(i, w)
		style := tabStyle
		if p == m.page || p == pageNodes && m.page == pageNodeDetail {
			style = activeTabStyle
			if m.focus == focusNavigation {
				style = focusedStyle
			}
		}
		width := lipgloss.Width(style.Render(label))
		if m.mouseY == 3 && m.mouseX >= x && m.mouseX < x+width {
			style = hoverStyle
		}
		items = append(items, style.Render(label))
		x += width + 1
	}
	return ansi.Truncate(strings.Join(items, " "), w, "")
}

func (m *Model) content() string {
	if m.menu != nil {
		return m.managementView()
	}
	if m.err != nil && m.conn == connError {
		return m.errorView()
	}
	if m.err != nil && (m.page == pageNodes || m.page == pageUsers || m.page == pageNodeDetail) {
		return m.section(pageNames[m.page], "Headscale API unavailable") + "\n\n" + errStyle.Render(m.formError())
	}
	switch m.page {
	case pageAccounts:
		return m.accountsView()
	case pageProviders:
		return m.providersView()
	case pageHeadscale:
		return m.managedView()
	case pagePolicy:
		return m.policyView()
	case pageKeys:
		return m.keysView()
	case pageDNS:
		return m.dnsView()
	case pageSettings:
		return m.settingsView()
	case pageStatus:
		return m.statusView()
	case pageNodes:
		return m.nodesView()
	case pageNodeDetail:
		return m.detailView()
	case pageUsers:
		return m.usersView()
	}
	return ""
}

func (m *Model) errorView() string {
	return m.section("Connection unavailable", "Cannot reach the Headdeck daemon") + "\n\n" +
		errStyle.Render(m.err.Error()) + "\n\n" +
		dimStyle.Render(m.tr("Socket  %s", m.socket)) + "\n\n" +
		m.tr("Press a to authorize your user through sudo.") + "\n" +
		m.tr("Start the service with sudo systemctl start headdeck.service") + "\n" +
		dimStyle.Render(m.tr("Or run hdkt daemon --config /var/lib/headdeck/settings.db"))
}

func (m *Model) footer(w int) string {
	var buttons []string
	x := m.layout().mainX
	left := x
	for i, action := range m.visibleActions() {
		label := m.actionLabel(action)
		style := buttonStyle
		if m.focus == focusActions && i == min(m.actionCursor, len(m.visibleActions())-1) {
			style = focusedStyle
		}
		width := lipgloss.Width(style.Render(label))
		if x-left+width > w {
			break
		}
		if m.mouseY == m.height-2 && m.mouseX >= x && m.mouseX < x+width {
			style = hoverStyle
		}
		buttons = append(buttons, style.Render(label))
		x += width + 1
	}
	line := strings.Join(buttons, " ")
	hint := dimStyle.Render(m.tr("Tab · switch focus"))
	if m.notice != "" {
		hint = okStyle.Render(m.tr(m.notice))
	}
	if space := w - lipgloss.Width(line) - lipgloss.Width(hint); space >= 2 {
		line += strings.Repeat(" ", space) + hint
	}
	return line
}
