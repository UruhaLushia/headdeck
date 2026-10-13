package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type settingsMsg struct {
	settings *protocol.Settings
	err      error
	gen      int
}
type settingSavedMsg struct {
	settings *protocol.Settings
	err      error
}

func (m *Model) loadSettings() tea.Cmd {
	if m.editKey != "" || m.resourceBusy || m.settingPopup != nil {
		return nil
	}
	m.settingsGen++
	gen := m.settingsGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		s, err := m.client.Settings(ctx)
		return settingsMsg{settings: s, err: err, gen: gen}
	}
}

func (m *Model) saveSetting() tea.Cmd {
	return m.updateSetting(m.editKey, m.editValue)
}

func (m *Model) updateSetting(key, value string) tea.Cmd {
	if m.page == pageHeadscale {
		return m.saveManagedValue(key, value)
	}
	if m.page == pageDNS {
		return m.saveDNSValue(key, value)
	}
	m.settingsGen++
	m.resourceBusy = true
	m.err = nil
	m.notice = ""
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		s, err := m.client.UpdateSettings(ctx, map[string]string{key: value})
		return settingSavedMsg{settings: s, err: err}
	}
}

func (m *Model) editSetting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearEditor()
		m.err = nil
		return m, nil
	case "enter":
		if m.resource != nil {
			return m, m.saveResource()
		}
		return m, m.saveSetting()
	}
	m.editValue, m.editCursor = editText(m.editValue, m.editCursor, msg)
	return m, nil
}

func (m *Model) beginSettingEdit() {
	item := m.currentSetting()
	if item == nil || !m.canEditSettings() {
		return
	}
	if m.page == pageDNS {
		m.dnsGen++
	}
	if m.page == pageHeadscale {
		m.managedGen++
		m.managedLoading = false
	}
	m.editKey, m.editValue = item.Key, item.Value
	m.editCursor = len([]rune(m.editValue))
	m.openSettingForm(item)
}

func (m *Model) settingsView() string {
	if m.settings == nil {
		return m.section("Settings", "Administrator access is required to manage settings.")
	}
	intro := m.section("Settings", "")
	if m.page == pageDNS {
		intro = m.section("DNS", "MagicDNS, nameservers and custom records")
	}
	if m.page == pageHeadscale {
		intro = m.section("Headscale", "Server, DERP and node lifecycle")
	}
	w := m.contentWidth()
	category := m.settingsCategory()
	if category != nil {
		if *category == "" {
			groups := m.settingsCategories()
			lines := []string{intro, ""}
			start, end := m.listWindow(len(groups), 9)
			for i := start; i < end; i++ {
				lines = append(lines, m.selectedRow(cell(m.tr(groups[i]), w-3)+"  ›", i))
			}
			if m.err != nil {
				lines = append(lines, "", errStyle.Render(m.formError()))
			} else if m.page == pageHeadscale && m.managed != nil && m.managed.Pending {
				lines = append(lines, "", warnStyle.Render(m.tr("Pending configuration · apply restarts Headscale")))
			}
			return strings.Join(lines, "\n")
		}
		title := "Settings"
		if m.page == pageHeadscale {
			title = "Headscale"
		}
		intro = m.section(m.tr(title)+" / "+m.tr(*category), "")
	}
	items := m.visibleSettings()
	labelWidth := min(34, max(12, (w-2)/2))
	lines := []string{intro, "", dimStyle.Render(cell(m.tr("SETTING"), labelWidth) + "  " + m.tr("VALUE"))}
	start, end := m.listWindow(len(items), 9)
	for i := start; i < end; i++ {
		item := items[i]
		value := item.Value
		if len(settingChoices(item.Key)) != 0 {
			value = m.choiceLabel(value) + "  ▾"
		}
		if item.Secret {
			value = m.tr("Not configured")
			if item.Configured {
				value = m.tr("[configured]")
			}
		} else if value == "" || value == "null" {
			value = "—"
		}
		label := m.tr(settingLabel(item.Key))
		if category != nil {
			if _, short, ok := strings.Cut(label, " · "); ok {
				label = short
			}
		}
		lines = append(lines, m.selectedRow(cell(label, labelWidth)+"  "+value, i))
	}
	lines = append(lines, "")
	if m.cursor >= 0 && m.cursor < len(items) {
		item := items[m.cursor]
		lines = append(lines, titleStyle.Render(m.tr(settingLabel(item.Key))), dimStyle.Render(m.tr(settingHelp(item.Key))))
	}
	state := okStyle.Render(m.tr("✓ All changes applied"))
	if m.page != pageSettings && m.settings.ReloadRequired {
		state = warnStyle.Render(m.tr("● Pending changes · press a to apply"))
	}
	if m.page == pageDNS && m.dns.Pending {
		message := "Pending DNS changes · apply restarts Headscale"
		if !m.dns.RestartRequired {
			message = "Pending records · apply without restarting"
		}
		state = warnStyle.Render(m.tr(message))
	}
	if m.page == pageDNS && !m.dns.Writable {
		state = warnStyle.Render(m.tr("Read-only · press s to authorize through sudo"))
	}
	if m.page == pageHeadscale && m.managed != nil {
		if m.managed.Pending {
			state = warnStyle.Render(m.tr("Pending configuration · apply restarts Headscale"))
		}
		if !m.managed.Writable {
			state = warnStyle.Render(m.tr("Read-only · press s to authorize through sudo"))
		}
	}
	if m.err != nil {
		state = errStyle.Render(m.formError())
	}
	lines = append(lines, "", state)
	return strings.Join(lines, "\n")
}
