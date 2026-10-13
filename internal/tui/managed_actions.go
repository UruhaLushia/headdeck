package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) managedActions() []uiAction {
	permission := "headscale.config.write"
	if m.page == pagePolicy {
		permission = "policy.write"
	}
	if !m.can(permission) {
		return []uiAction{{"Refresh", "r"}}
	}
	writable, pending := false, false
	if m.page == pageHeadscale && m.managed != nil {
		writable, pending = m.managed.Writable, m.managed.Pending
	}
	if m.page == pagePolicy && m.policy != nil {
		writable, pending = m.policy.Writable, m.policy.Pending
	}
	if !writable {
		return []uiAction{{"Authorize", "s"}, {"Discard", "x"}, {"Refresh", "r"}}
	}
	actions := []uiAction{{"Edit", "enter"}}
	if item := m.currentSetting(); item != nil && len(settingChoices(item.Key)) > 0 {
		actions[0] = uiAction{"Select", "enter"}
	}
	if m.page == pagePolicy {
		items := m.policyItems()
		if m.cursor < len(items) && !items[m.cursor].Add {
			actions = append(actions, uiAction{"Delete", "d"})
		}
	}
	if pending {
		actions = append(actions, uiAction{"Apply", "a"}, uiAction{"Discard", "x"})
	}
	return append(actions, uiAction{"Refresh", "r"})
}

func (m *Model) managedKey(key string) (bool, tea.Cmd) {
	if m.page != pageHeadscale && m.page != pagePolicy || m.dialogOpen() {
		return false, nil
	}
	prefix, permission, title := "config", "headscale.config.write", "Headscale configuration"
	writable, pending, revision := false, false, ""
	if m.page == pageHeadscale && m.managed != nil {
		writable, pending, revision = m.managed.Writable, m.managed.Pending, m.managed.Revision
	}
	if m.page == pagePolicy {
		prefix, permission, title = "policy", "policy.write", "ACL policy"
		if m.policy != nil {
			writable, pending, revision = m.policy.Writable, m.policy.Pending, m.policy.Revision
		}
	}
	switch key {
	case "enter":
		if !writable || !m.can(permission) {
			return true, nil
		}
		if m.page == pagePolicy {
			m.beginPolicyEdit()
		} else if item := m.currentSetting(); item != nil && len(settingChoices(item.Key)) > 0 {
			m.openSettingChoices()
		} else {
			m.beginSettingEdit()
		}
		return true, nil
	case "d", "delete":
		if m.page == pagePolicy && writable && m.can(permission) {
			items := m.policyItems()
			if m.cursor < len(items) && !items[m.cursor].Add {
				item := items[m.cursor]
				m.beginResource(resourceAction{kind: "policy.delete", label: "Delete policy item", target: item.Title, confirm: item.Title, id: fmt.Sprint(m.cursor), value: revision, help: "This removes the item from the policy draft."})
			}
		}
		return true, nil
	case "a":
		if pending && writable && m.can(permission) {
			help := "Headscale will restart briefly to apply this configuration."
			if m.page == pagePolicy && m.policy.Mode == "database" {
				help = "The ACL policy will be applied without restarting Headscale."
			}
			m.beginResource(resourceAction{kind: prefix + ".apply", label: "Apply " + title, target: title, confirm: title, value: revision, help: help})
		}
		return true, nil
	case "x":
		if m.can(permission) {
			m.beginResource(resourceAction{kind: prefix + ".discard", label: "Discard draft", target: title, confirm: title, help: "All pending changes will be discarded."})
		}
		return true, nil
	case "s":
		if m.can(permission) {
			return true, m.authorizeDNS()
		}
		return true, nil
	}
	return false, nil
}
