package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) dnsKey(key string) (bool, tea.Cmd) {
	if m.page != pageDNS || m.dialogOpen() {
		return false, nil
	}
	switch key {
	case "enter":
		if m.dns == nil || !m.dns.Writable {
			return true, nil
		}
		if item := m.currentSetting(); item != nil && len(settingChoices(item.Key)) > 0 {
			m.openSettingChoices()
		} else {
			m.beginSettingEdit()
		}
		return true, nil
	case "a":
		if m.dns != nil && m.dns.Pending && m.can("dns.write") && m.status != nil {
			help := "Headscale will restart briefly to apply DNS."
			if !m.dns.RestartRequired {
				help = "DNS records will be applied without restarting Headscale."
			}
			m.beginResource(resourceAction{kind: "dns.apply", label: "Apply DNS", target: m.status.ServiceControl.Unit, confirm: m.status.ServiceControl.Unit, help: help})
		}
		return true, nil
	case "x":
		if m.can("dns.write") {
			m.beginResource(resourceAction{kind: "dns.discard", label: "Discard DNS draft", confirm: "DNS", target: "DNS", help: "All pending DNS changes will be discarded."})
		}
		return true, nil
	case "d", "delete":
		item := m.currentSetting()
		if item == nil || !m.can("dns.write") || !m.dns.Writable {
			return true, nil
		}
		if strings.Contains(item.Key, ":") && !strings.HasPrefix(item.Key, "dns.add:") {
			m.beginResource(resourceAction{kind: "dns.delete", label: "Delete DNS entry", id: item.Key, target: item.Value, confirm: item.Value, help: "This removes the entry from the DNS draft."})
		}
		return true, nil
	case "s":
		return true, m.authorizeDNS()
	}
	return false, nil
}

func (m *Model) deleteDNS(key string) tea.Cmd {
	if m.dns == nil {
		return nil
	}
	kind, id, _ := strings.Cut(strings.TrimPrefix(key, "dns."), ":")
	i, err := strconv.Atoi(id)
	c := m.dns.Config
	switch kind {
	case "global":
		if err != nil || i < 0 || i >= len(c.Nameservers.Global) {
			return nil
		}
		c.Nameservers.Global = append(append([]string{}, c.Nameservers.Global[:i]...), c.Nameservers.Global[i+1:]...)
	case "search":
		if err != nil || i < 0 || i >= len(c.SearchDomains) {
			return nil
		}
		c.SearchDomains = append(append([]string{}, c.SearchDomains[:i]...), c.SearchDomains[i+1:]...)
	case "record":
		if err != nil || i < 0 || i >= len(c.ExtraRecords) {
			return nil
		}
		c.ExtraRecords = append(append([]protocol.DNSRecord{}, c.ExtraRecords[:i]...), c.ExtraRecords[i+1:]...)
	case "split":
		c.Nameservers.Split = make(map[string][]string)
		for k, v := range m.dns.Config.Nameservers.Split {
			if k != id {
				c.Nameservers.Split[k] = v
			}
		}
	default:
		return nil
	}
	req := protocol.DNSUpdate{Config: c, Revision: m.dns.Revision}
	m.resourceBusy = true
	m.dnsGen++
	gen := m.dnsGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.SaveDNS(ctx, req)
		return dnsMsg{settings: out, err: err, saved: true, gen: gen}
	}
}

func dnsLabel(key string) string {
	switch key {
	case "dns.magic_dns":
		return "MagicDNS"
	case "dns.base_domain":
		return "Tailnet domain"
	case "dns.override_local_dns":
		return "Override local DNS"
	case "dns.add:global":
		return "+ Add nameserver"
	case "dns.add:split":
		return "+ Add split DNS"
	case "dns.add:search":
		return "+ Add search domain"
	case "dns.add:record":
		return "+ Add DNS record"
	}
	kind, _, _ := strings.Cut(strings.TrimPrefix(key, "dns."), ":")
	return map[string]string{"global": "Global nameserver", "split": "Split DNS", "search": "Search domain", "record": "DNS record"}[kind]
}

func dnsHelp(key string) string {
	if len(settingChoices(key)) > 0 {
		return "Press Enter or click to choose an option."
	}
	if strings.Contains(key, "split") {
		return "Edit the domain and its nameservers."
	}
	if strings.Contains(key, "record") {
		return "Edit the record name and its IP address."
	}
	if strings.Contains(key, "global") {
		return "Enter an IP address or an HTTPS DNS resolver URL."
	}
	return "Enter a fully qualified domain without a trailing dot."
}

func (m *Model) dnsActions() []uiAction {
	actions := []uiAction{{"Refresh", "r"}}
	if !m.can("dns.write") {
		return actions
	}
	if m.dns == nil {
		return append([]uiAction{{"Authorize", "s"}}, append(actions, uiAction{"Discard", "x"})...)
	}
	if !m.dns.Writable {
		return append([]uiAction{{"Authorize", "s"}}, actions...)
	}
	actions = []uiAction{{"Edit", "enter"}}
	if item := m.currentSetting(); item != nil {
		if len(settingChoices(item.Key)) > 0 {
			actions[0] = uiAction{"Select", "enter"}
		}
		if strings.Contains(item.Key, ":") && !strings.HasPrefix(item.Key, "dns.add:") {
			actions = append(actions, uiAction{"Delete", "d"})
		}
	}
	if m.dns.Pending {
		actions = append(actions, uiAction{"Apply", "a"}, uiAction{"Discard", "x"})
	}
	return append(actions, uiAction{"Refresh", "r"})
}
