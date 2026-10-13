package tui

import (
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func settingCategory(key string) string {
	switch {
	case key == "server.diagnostics":
		return "Web"
	case strings.HasPrefix(key, "features."):
		return "Service control"
	case strings.HasPrefix(key, "server.web.github."):
		return "GitHub login"
	case strings.HasPrefix(key, "server.web.oidc."):
		return "OIDC login"
	case strings.HasPrefix(key, "server.web."):
		return "Web"
	case strings.HasPrefix(key, "server.log"), strings.HasPrefix(key, "hs.log"):
		return "Logging"
	case strings.HasPrefix(key, "hs.prefixes."):
		return "IP allocation"
	case strings.HasPrefix(key, "hs.derp."):
		return "DERP"
	case strings.HasPrefix(key, "hs.node."), strings.HasPrefix(key, "hs.ephemeral"):
		return "Node lifecycle"
	default:
		return "Server"
	}
}

func (m *Model) settingsCategory() *string {
	if m.page == pageSettings {
		return &m.settingsGroup
	}
	if m.page == pageHeadscale {
		return &m.managedGroup
	}
	return nil
}

func (m *Model) settingItems() []protocol.Setting {
	if m.page == pageHeadscale && m.managed != nil {
		return m.managed.Items
	}
	if m.page == pageDNS {
		if settings := m.dnsSettings(); settings != nil {
			return settings.Items
		}
		return nil
	}
	if m.page == pageSettings && m.settings != nil {
		return m.settings.Items
	}
	return nil
}

func (m *Model) settingsCategories() []string {
	present := make(map[string]bool)
	for _, item := range m.settingItems() {
		present[settingCategory(item.Key)] = true
	}
	var groups []string
	for _, group := range []string{"Server", "IP allocation", "DERP", "Node lifecycle", "Web", "GitHub login", "Service control", "OIDC login", "Logging"} {
		if present[group] {
			groups = append(groups, group)
		}
	}
	return groups
}

func (m *Model) visibleSettings() []protocol.Setting {
	items := m.settingItems()
	category := m.settingsCategory()
	if category == nil {
		return items
	}
	var visible []protocol.Setting
	for _, item := range items {
		if settingCategory(item.Key) == *category {
			visible = append(visible, item)
		}
	}
	return visible
}

func (m *Model) settingsCategoryKey(key string) bool {
	category := m.settingsCategory()
	if category == nil {
		return false
	}
	if *category == "" && key == "enter" {
		groups := m.settingsCategories()
		if m.cursor >= 0 && m.cursor < len(groups) {
			*category = groups[m.cursor]
			m.cursor, m.listOffset = 0, 0
		}
		return true
	}
	if *category != "" && (key == "esc" || key == "backspace" || key == "left") {
		groups := m.settingsCategories()
		for i, group := range groups {
			if group == *category {
				m.cursor = i
			}
		}
		*category, m.listOffset = "", 0
		return true
	}
	return false
}
