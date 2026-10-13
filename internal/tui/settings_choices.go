package tui

import (
	"slices"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func settingChoices(key string) []string {
	if choices := managedChoices(key); choices != nil {
		return choices
	}
	switch key {
	case "features.headscale_service_control",
		"server.web.password_login",
		"server.web.oidc.enabled", "server.web.oidc.auto_register", "server.web.oidc.disable_token_login",
		"server.web.github.enabled", "server.web.github.auto_register", "server.web.github.disable_token_login",
		"server.diagnostics", "server.log_json", "server.web.enabled", "server.web.cookie_secure", "dns.magic_dns", "dns.override_local_dns":
		return []string{"true", "false"}
	case "features.service_control_driver":
		return []string{"auto", "systemd", "none"}
	case "server.web.default_role":
		return []string{"pending", "member"}
	case "server.log_level":
		return []string{"debug", "info", "warn", "error"}
	}
	return nil
}

func (m *Model) currentSetting() *protocol.Setting {
	items := m.visibleSettings()
	if m.cursor >= 0 && m.cursor < len(items) {
		return &items[m.cursor]
	}
	return nil
}

func (m *Model) choiceLabel(value string) string {
	switch value {
	case "accept":
		return m.tr("Allow")
	case "check":
		return m.tr("Require reauthentication")
	case "true":
		return m.tr("Enabled")
	case "false":
		return m.tr("Disabled")
	case "pending":
		return m.tr("pending")
	case "member":
		return m.tr("member")
	}
	return value
}

func (m *Model) openSettingChoices() {
	item := m.currentSetting()
	if item == nil || !m.canEditSettings() {
		return
	}
	choices := settingChoices(item.Key)
	if len(choices) == 0 {
		return
	}
	m.settingPopup = &settingSelection{key: item.Key, value: item.Value, choices: choices,
		cursor: max(0, slices.Index(choices, item.Value)), button: -1}
	m.settingsGen++
	if m.page == pageDNS {
		m.dnsGen++
	}
	if m.page == pageHeadscale {
		m.managedGen++
		m.managedLoading = false
	}
	m.focus = focusContent
	m.err = nil
	m.notice = ""
}
