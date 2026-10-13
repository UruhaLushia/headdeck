package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, m.quit()
	}
	if m.resourceBusy {
		return m, nil
	}
	if m.confirmation != nil {
		return m.confirmationKey(msg)
	}
	if m.settingPopup != nil {
		return m.settingPopupKey(msg)
	}
	if m.objectForm != nil {
		return m.objectKey(msg)
	}
	if m.page == pageAccounts && !m.dialogOpen() && msg.String() == "n" && m.can("accounts.write") {
		m.accountForm("account.create", "")
		return m, nil
	}
	if m.page == pageProviders && !m.dialogOpen() && msg.String() == "n" && m.can("accounts.write") {
		m.providerForm("")
		return m, nil
	}
	if m.page == pageKeys && !m.dialogOpen() && msg.String() == "n" {
		m.newKeyForm()
		return m, nil
	}
	if !m.dialogOpen() && m.focus == focusContent && m.settingsCategoryKey(msg.String()) {
		return m, nil
	}
	if handled, cmd := m.focusKey(msg); handled {
		return m, cmd
	}
	if m.editKey != "" {
		return m.editSetting(msg)
	}
	if m.menu != nil {
		return m.managementKey(msg)
	}
	if handled, cmd := m.dnsKey(msg.String()); handled {
		return m, cmd
	}
	if handled, cmd := m.managedKey(msg.String()); handled {
		return m, cmd
	}
	if !m.searching && msg.String() == "n" && m.page == pageUsers && m.can("users.write") {
		m.beginResource(resourceAction{kind: "user.create", label: "Create user", help: "Enter a local Headscale username."})
		return m, nil
	}
	if !m.searching && msg.String() == "m" && m.page == pageAccounts {
		return m, m.loadAccountSecurity()
	}
	if !m.searching && msg.String() == "m" && m.page == pageProviders {
		m.providerMenu()
		return m, nil
	}
	if !m.searching && msg.String() == "m" && (m.page == pageStatus || m.page == pageNodes || m.page == pageNodeDetail || m.page == pageUsers || m.page == pageKeys || m.page == pageSettings) {
		m.openManagement()
		return m, nil
	}
	if m.page == pageNodeDetail {
		switch msg.String() {
		case "up", "k":
			m.scrollDetail(-1)
			return m, nil
		case "down", "j":
			m.scrollDetail(1)
			return m, nil
		case "pgup":
			m.scrollDetail(-max(1, m.height-13))
			return m, nil
		case "pgdown":
			m.scrollDetail(max(1, m.height-13))
			return m, nil
		case "home":
			m.detailOffset = 0
			return m, nil
		case "end":
			m.scrollDetail(len(m.detailRows()))
			return m, nil
		case "left":
			return m.press("esc")
		}
	}
	if !m.searching && msg.String() == "a" && m.conn == connError {
		return m, m.authorize()
	}
	if msg.String() == "enter" && m.page == pageSettings {
		if item := m.currentSetting(); item != nil && len(settingChoices(item.Key)) != 0 {
			m.openSettingChoices()
			return m, nil
		}
		m.beginSettingEdit()
		return m, nil
	}

	if m.searching {
		switch msg.Type {
		case tea.KeyEsc:
			m.filter = ""
			m.searching = false
		case tea.KeyEnter:
			m.searching = false
		case tea.KeyBackspace:
			if len(m.filter) > 0 {
				runes := []rune(m.filter)
				m.filter = string(runes[:len(runes)-1])
			}
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
		case tea.KeySpace:
			m.filter += " "
		}
		m.clampCursor()
		return m, nil
	}
	if key := msg.String(); len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		index := (int(key[0]-'0') + 9) % 10
		m.focus, m.page, m.cursor, m.listOffset = focusContent, navigationPages[index], 0, 0
		m.filter = ""
		return m, m.pageLoad()
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, m.quit()
	case "?":
		m.focus = focusNavigation
		m.page = pageStatus
		return m, nil
	case "/":
		if m.page == pageNodes || m.page == pageUsers || m.overviewNodes() {
			m.focus = focusContent
			m.searching = true
			m.filter = ""
			m.cursor = 0
		}
		return m, nil
	case "esc":
		if m.page == pageNodeDetail {
			m.page = pageNodes
		}
		m.filter = ""
		m.clampCursor()
		return m, nil
	case "home":
		m.cursor = 0
		return m, nil
	case "end":
		m.cursor = max(0, m.rowCount()-1)
		return m, nil
	case "pgup", "pgdown":
		step := max(1, m.height-16)
		if m.page == pageSettings {
			step = max(1, m.height-19)
		}
		if msg.String() == "pgup" {
			step = -step
		}
		m.cursor = max(0, min(max(0, m.rowCount()-1), m.cursor+step))
		return m, nil
	case "r":
		return m, m.refresh()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < m.rowCount()-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		if m.page == pageAccounts {
			return m, m.loadAccountSecurity()
		}
		if m.page == pageProviders {
			m.providerMenu()
			return m, nil
		}
		if m.page == pageKeys {
			m.openManagement()
			return m, nil
		}
		if m.page == pageUsers && m.can("users.write") {
			m.openManagement()
			return m, nil
		}
		if m.page == pageNodes || m.overviewNodes() {
			if node, ok := m.selectedNode(); ok {
				m.page = pageNodeDetail
				m.detailOffset = 0
				return m, m.loadDetail(node.ID)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) pageLoad() tea.Cmd {
	switch m.page {
	case pageAccounts:
		return tea.Batch(m.loadAccounts(), m.loadUsers())
	case pageProviders:
		return m.loadProviders()
	case pageDNS:
		return m.loadDNS()
	case pageHeadscale:
		return m.loadManaged()
	case pagePolicy:
		return m.loadPolicy()
	case pageKeys:
		return m.loadKeys()
	case pageStatus:
		return tea.Batch(m.loadStatus(), m.loadKeys())
	case pageNodes, pageNodeDetail:
		return m.loadNodes()
	case pageSettings:
		return m.loadSettings()
	case pageUsers:
		return m.loadUsers()
	}
	return nil
}

func (m *Model) refresh() tea.Cmd {
	m.err = nil
	m.conn = connConnecting
	return tea.Batch(m.connect(), m.pageLoad())
}

func (m *Model) rowCount() int {
	if m.menu != nil {
		return len(m.menu)
	}
	switch m.page {
	case pageStatus:
		if m.overviewNodes() {
			return len(m.visibleNodes())
		}
	case pageAccounts:
		return len(m.accounts)
	case pageProviders:
		return len(m.providers)
	case pageKeys:
		return len(m.keys)
	case pagePolicy:
		return len(m.policyItems())
	case pageDNS:
		if s := m.dnsSettings(); s != nil {
			return len(s.Items)
		}
	case pageHeadscale, pageSettings:
		if category := m.settingsCategory(); *category == "" {
			return len(m.settingsCategories())
		}
		return len(m.visibleSettings())
	case pageNodes, pageNodeDetail:
		return len(m.visibleNodes())
	case pageUsers:
		return len(m.visibleUsers())
	}
	return 0
}

func (m *Model) clampCursor() {
	if n := m.rowCount(); m.cursor >= n {
		m.cursor = max(0, n-1)
	}
}
