package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

type dnsMsg struct {
	settings *protocol.DNSSettings
	err      error
	saved    bool
	gen      int
}

func (m *Model) loadDNS() tea.Cmd {
	if m.dialogOpen() || m.resourceBusy || m.dnsLoading {
		return nil
	}
	m.dnsLoading = true
	m.dnsGen++
	gen := m.dnsGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.DNS(ctx)
		return dnsMsg{settings: out, err: err, gen: gen}
	}
}

func (m *Model) dnsSettings() *protocol.Settings {
	if m.dns == nil {
		return nil
	}
	c := m.dns.Config
	out := &protocol.Settings{ReloadRequired: m.dns.Pending}
	add := func(key, value string) { out.Items = append(out.Items, protocol.Setting{Key: key, Value: value}) }
	add("dns.magic_dns", strconv.FormatBool(c.MagicDNS))
	add("dns.base_domain", c.BaseDomain)
	add("dns.override_local_dns", strconv.FormatBool(c.OverrideLocalDNS))
	for i, ns := range c.Nameservers.Global {
		add(fmt.Sprintf("dns.global:%d", i), ns)
	}
	add("dns.add:global", "")
	domains := make([]string, 0, len(c.Nameservers.Split))
	for domain := range c.Nameservers.Split {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		add("dns.split:"+domain, domain+" = "+strings.Join(c.Nameservers.Split[domain], ", "))
	}
	add("dns.add:split", "")
	for i, domain := range c.SearchDomains {
		add(fmt.Sprintf("dns.search:%d", i), domain)
	}
	add("dns.add:search", "")
	for i, r := range c.ExtraRecords {
		add(fmt.Sprintf("dns.record:%d", i), r.Name+" = "+r.Value)
	}
	add("dns.add:record", "")
	return out
}

func (m *Model) dnsView() string {
	if m.dns == nil {
		view := m.section("DNS", "MagicDNS, nameservers and custom records")
		if m.err != nil {
			view += "\n\n" + errStyle.Render(m.formError())
		}
		return view
	}
	saved := m.settings
	m.settings = m.dnsSettings()
	view := m.settingsView()
	m.settings = saved
	return view
}

func (m *Model) saveDNSValue(key, value string) tea.Cmd {
	if m.dns == nil || !m.dns.Writable || !m.can("dns.write") {
		return nil
	}
	config := m.dns.Config
	// Detach the draft before editing maps and slices shared with the current view.
	config.Nameservers.Global = append([]string{}, config.Nameservers.Global...)
	config.SearchDomains = append([]string{}, config.SearchDomains...)
	config.ExtraRecords = append([]protocol.DNSRecord{}, config.ExtraRecords...)
	split := map[string][]string{}
	for k, v := range config.Nameservers.Split {
		split[k] = append([]string{}, v...)
	}
	config.Nameservers.Split = split
	if err := editDNS(&config, key, strings.TrimSpace(value)); err != nil {
		m.err = err
		return nil
	}
	req := protocol.DNSUpdate{Config: config, Revision: m.dns.Revision}
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

func editDNS(c *protocol.DNSConfig, key, value string) error {
	switch key {
	case "dns.magic_dns":
		c.MagicDNS = value == "true"
		return nil
	case "dns.override_local_dns":
		c.OverrideLocalDNS = value == "true"
		return nil
	case "dns.base_domain":
		c.BaseDomain = value
		return nil
	}
	kind, id, ok := strings.Cut(strings.TrimPrefix(key, "dns."), ":")
	if !ok {
		return fmt.Errorf("unknown DNS field")
	}
	if kind == "add" {
		kind, id = id, ""
	}
	i, err := strconv.Atoi(id)
	switch kind {
	case "global":
		if id == "" {
			c.Nameservers.Global = append(c.Nameservers.Global, value)
		} else if err == nil && i >= 0 && i < len(c.Nameservers.Global) {
			c.Nameservers.Global[i] = value
		} else {
			return fmt.Errorf("invalid nameserver selection")
		}
	case "search":
		if id == "" {
			c.SearchDomains = append(c.SearchDomains, value)
		} else if err == nil && i >= 0 && i < len(c.SearchDomains) {
			c.SearchDomains[i] = value
		} else {
			return fmt.Errorf("invalid search domain selection")
		}
	case "split":
		domain, servers, ok := strings.Cut(value, "=")
		domain = strings.TrimSpace(domain)
		if !ok || domain == "" {
			return fmt.Errorf("use domain = nameserver, nameserver")
		}
		if domain != id {
			if _, exists := c.Nameservers.Split[domain]; exists {
				return fmt.Errorf("split DNS domain already exists")
			}
			delete(c.Nameservers.Split, id)
		}
		c.Nameservers.Split[domain] = splitValues(servers)
	case "record":
		name, address, ok := strings.Cut(value, "=")
		if !ok {
			return fmt.Errorf("use name = IPv4 or IPv6 address")
		}
		r := protocol.DNSRecord{Name: strings.TrimSpace(name), Value: strings.TrimSpace(address), Type: "A"}
		if strings.Contains(r.Value, ":") {
			r.Type = "AAAA"
		}
		if id == "" {
			c.ExtraRecords = append(c.ExtraRecords, r)
		} else if err == nil && i >= 0 && i < len(c.ExtraRecords) {
			c.ExtraRecords[i] = r
		} else {
			return fmt.Errorf("invalid record selection")
		}
	default:
		return fmt.Errorf("unknown DNS field")
	}
	return nil
}
