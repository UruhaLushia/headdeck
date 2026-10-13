package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type uiAction struct{ label, key string }

func (m *Model) visibleActions() []uiAction {
	width := 0
	var actions []uiAction
	for _, action := range m.actions() {
		size := lipgloss.Width(buttonStyle.Render(m.actionLabel(action)))
		if width+size > m.layout().mainWidth {
			break
		}
		actions = append(actions, action)
		width += size + 1
	}
	return actions
}

func (m *Model) actions() []uiAction {
	if m.objectForm != nil || m.resourceBusy || m.settingPopup != nil || m.confirmation != nil {
		return []uiAction{}
	}
	if m.menu != nil {
		return []uiAction{{"Select", "enter"}, {"Cancel", "esc"}}
	}
	if m.editKey != "" {
		if m.resource != nil && m.resource.confirm != "" {
			return []uiAction{{"Confirm", "enter"}, {"Cancel", "esc"}}
		}
		return []uiAction{{"Save", "enter"}, {"Cancel", "esc"}}
	}
	if m.searching {
		return []uiAction{{"Done", "enter"}, {"Clear", "esc"}}
	}
	actions := []uiAction{{"Refresh", "r"}}
	switch {
	case m.page == pageAccounts || m.page == pageProviders:
		if m.can("accounts.write") {
			label := "New account"
			if m.page == pageProviders {
				label = "Add provider"
			}
			actions = append([]uiAction{{label, "n"}, {"Manage", "m"}}, actions...)
		}
	case m.conn == connError:
		actions = append(actions, uiAction{"Authorize", "a"})
	case m.page == pageStatus:
		if m.overviewNodes() {
			actions = append([]uiAction{{"Open", "enter"}, {"Filter", "/"}}, actions...)
		}
		if m.can("headscale.restart") && m.status != nil && m.status.ServiceControl.Supported {
			actions = append([]uiAction{{"Headscale service", "m"}}, actions...)
		}
	case m.page == pageNodes:
		actions = []uiAction{{"Open", "enter"}, {"Filter", "/"}, {"Refresh", "r"}}
		if m.can("nodes.write") || m.can("nodes.delete") || m.can("routes.write") {
			actions = append([]uiAction{{"Open", "enter"}, {"Manage", "m"}}, actions[1:]...)
		}
	case m.page == pageKeys:
		if m.can("keys.write") {
			actions = append([]uiAction{{"Create key", "n"}}, actions...)
			if len(m.keys) > 0 {
				actions = append([]uiAction{{"Manage", "m"}}, actions...)
			}
		}
	case m.page == pageUsers:
		if m.can("users.write") {
			actions = []uiAction{{"New user", "n"}}
			if len(m.visibleUsers()) > 0 {
				actions = append(actions, uiAction{"Manage", "m"})
			}
			actions = append(actions, uiAction{"Refresh", "r"})
		}
		actions = append(actions, uiAction{"Filter", "/"})
	case m.page == pageSettings:
		actions = []uiAction{{"Edit", "enter"}}
		if item := m.currentSetting(); item != nil && len(settingChoices(item.Key)) != 0 {
			actions = []uiAction{{"Select", "enter"}}
		}
		actions = append(actions, uiAction{"Refresh", "r"})
	case m.page == pageDNS:
		actions = m.dnsActions()
	case m.page == pageHeadscale || m.page == pagePolicy:
		actions = m.managedActions()
	case m.page == pageNodeDetail:
		if m.can("nodes.write") || m.can("nodes.delete") || m.can("routes.write") {
			actions = append([]uiAction{{"Manage", "m"}}, actions...)
		}
		actions = append(actions, uiAction{"Back", "esc"})
	}
	if category := m.settingsCategory(); category != nil {
		if *category == "" {
			if actions[0].key == "enter" {
				actions[0] = uiAction{"Open", "enter"}
			} else {
				actions = append([]uiAction{{"Open", "enter"}}, actions...)
			}
		} else {
			actions = append(actions, uiAction{"Back", "esc"})
		}
	}
	return append(actions, uiAction{"Quit", "q"})
}

func (m *Model) press(key string) (tea.Model, tea.Cmd) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+s":
		msg = tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+t":
		msg = tea.KeyMsg{Type: tea.KeyCtrlT}
	}
	return m.handleKey(msg)
}

func (m *Model) switchPage(direction int) tea.Cmd {
	index := 0
	for i, p := range navigationPages {
		if p == m.page || p == pageNodes && m.page == pageNodeDetail {
			index = i
			break
		}
	}
	m.page = navigationPages[(index+direction+len(navigationPages))%len(navigationPages)]
	m.cursor, m.listOffset, m.filter = 0, 0, ""
	m.searching = false
	return m.pageLoad()
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	l := m.layout()
	m.mouseX, m.mouseY = msg.X, msg.Y
	if m.resourceBusy {
		return m, nil
	}
	if m.confirmation != nil {
		return m.confirmationMouse(msg)
	}
	if m.settingPopup != nil {
		return m.settingPopupMouse(msg)
	}
	if m.objectForm != nil {
		return m.objectMouse(msg)
	}
	if m.menu != nil && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
		step := 1
		if msg.Button == tea.MouseButtonWheelUp {
			step = -1
		}
		m.menuCursor = max(0, min(len(m.menu)-1, m.menuCursor+step))
		return m, nil
	}
	if msg.Action == tea.MouseActionRelease {
		m.scrollDragging = false
	}
	if m.editKey == "" && !m.searching && m.conn == connReady &&
		m.isListPage() {
		reserved := 6
		if m.page == pageSettings || m.page == pageDNS || m.page == pageHeadscale {
			reserved = 9
		}
		start, end := m.listWindow(m.rowCount(), reserved)
		count := end - start
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft &&
			msg.X == l.contentX+l.contentWidth-1 && msg.Y >= l.listY && msg.Y < l.listY+count && m.rowCount() > count {
			m.scrollDragging = true
		}
		if m.scrollDragging && msg.Button == tea.MouseButtonLeft {
			position := max(0, min(count-1, msg.Y-l.listY))
			m.listOffset = position * (m.rowCount() - count) / max(1, count-1)
			m.cursor = max(m.listOffset, min(m.cursor, m.listOffset+count-1))
			return m, nil
		}
	}
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m.press("esc")
	}
	if msg.Button == tea.MouseButtonWheelLeft && !m.dialogOpen() {
		return m, m.switchPage(-1)
	}
	if msg.Button == tea.MouseButtonWheelRight && !m.dialogOpen() {
		return m, m.switchPage(1)
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		if l.sidebar > 0 && msg.X < l.mainX && msg.Y >= 5 || l.sidebar == 0 && msg.Y == 3 {
			step := 1
			if msg.Button == tea.MouseButtonWheelUp {
				step = -1
			}
			return m, m.switchPage(step)
		}
		if m.editKey == "" && msg.X >= l.contentX && msg.Y >= 6 && msg.Y < m.height-3 {
			m.focus = focusContent
			step := 3
			if msg.Button == tea.MouseButtonWheelUp {
				step = -3
			}
			if m.page == pageNodeDetail {
				m.scrollDetail(step)
				return m, nil
			}
			reserved := 6
			if m.page == pageSettings || m.page == pageDNS || m.page == pageHeadscale {
				reserved = 9
			}
			start, end := m.listWindow(m.rowCount(), reserved)
			count := max(1, end-start)
			m.listOffset = max(0, min(m.rowCount()-count, start+step))
			m.cursor = max(m.listOffset, min(m.cursor, m.listOffset+count-1))
		}
		return m, nil
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if i, hit := m.navigationHit(msg.X, msg.Y); hit && !m.dialogOpen() {
		m.focus = focusNavigation
		m.editKey, m.editValue, m.filter = "", "", ""
		m.searching = false
		m.page, m.cursor, m.listOffset = navigationPages[i], 0, 0
		return m, m.pageLoad()
	}
	if msg.Y == l.footerY {
		x := l.mainX
		for _, action := range m.visibleActions() {
			width := lipgloss.Width(buttonStyle.Render(m.actionLabel(action)))
			if x-l.mainX+width > l.mainWidth {
				break
			}
			if msg.X >= x && msg.X < x+width {
				m.focus = focusContent
				return m.press(action.key)
			}
			x += width + 1
		}
	}
	if m.menu != nil {
		index := msg.Y - l.listY
		if msg.X >= l.contentX && msg.X < l.contentX+l.contentWidth && index >= 0 && index < len(m.menu) {
			m.menuCursor = index
			m.focus = focusContent
			return m.press("enter")
		}
		return m, nil
	}
	if m.conn != connReady || m.searching || msg.X < l.contentX || msg.X >= l.contentX+l.contentWidth {
		return m, nil
	}
	if !m.isListPage() {
		return m, nil
	}
	reserved := 6
	if m.page == pageSettings || m.page == pageDNS || m.page == pageHeadscale {
		reserved = 9
	}
	start, end := m.listWindow(m.rowCount(), reserved)
	index := start + msg.Y - l.listY
	if msg.Y < l.listY || index >= end {
		return m, nil
	}
	now := time.Now()
	m.focus = focusContent
	double := m.lastClickPage == m.page && m.lastClickRow == index && now.Sub(m.lastClick) < 350*time.Millisecond
	m.cursor, m.lastClickRow, m.lastClickPage, m.lastClick = index, index, m.page, now
	if item := m.currentSetting(); item != nil && len(settingChoices(item.Key)) != 0 {
		m.openSettingChoices()
		return m, nil
	}
	if double {
		m.lastClick = time.Time{}
		return m.press("enter")
	}
	return m, nil
}
