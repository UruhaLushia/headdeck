package tui

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type managedMsg struct {
	settings *protocol.ManagedConfig
	err      error
	gen      int
	saved    bool
}

func (m *Model) loadManaged() tea.Cmd {
	if m.dialogOpen() || m.resourceBusy || m.managedLoading {
		return nil
	}
	m.managedLoading = true
	m.managedGen++
	gen := m.managedGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.ManagedConfig(ctx)
		return managedMsg{settings: out, err: err, gen: gen}
	}
}

func (m *Model) saveManagedValue(key, value string) tea.Cmd {
	if !m.canEditSettings() {
		return nil
	}
	m.managedGen++
	gen := m.managedGen
	req := protocol.ManagedConfigUpdate{Revision: m.managed.Revision, Values: map[string]string{key: value}}
	m.resourceBusy = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.SaveManagedConfig(ctx, req)
		return managedMsg{settings: out, err: err, gen: gen, saved: true}
	}
}

func (m *Model) managedUpdated(msg managedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.managedGen {
		return m, nil
	}
	m.managedLoading = false
	m.err = msg.err
	if msg.saved {
		m.resourceBusy = false
	}
	if msg.err == nil {
		m.managed = msg.settings
		m.clampCursor()
		if msg.saved {
			m.clearEditor()
			m.notice = "Changes saved"
		}
	}
	return m, nil
}

func (m *Model) managedView() string {
	if m.managed == nil {
		view := m.section("Headscale", "Server, DERP and node lifecycle")
		if m.err != nil {
			view += "\n\n" + errStyle.Render(m.formError())
		}
		return view
	}
	saved := m.settings
	m.settings = &protocol.Settings{Items: m.managed.Items, ReloadRequired: m.managed.Pending}
	view := m.settingsView()
	m.settings = saved
	return view
}

func managedChoices(key string) []string {
	switch key {
	case "hs.derp.server.enabled", "hs.derp.server.verify_clients", "hs.derp.server.automatically_add_embedded_derp_region",
		"hs.derp.auto_update_enabled", "hs.disable_check_updates", "hs.logtail.enabled", "hs.taildrop.enabled", "hs.auto_update.enabled":
		return []string{"true", "false"}
	case "hs.prefixes.allocation":
		return []string{"sequential", "random"}
	case "hs.log.level":
		return []string{"panic", "fatal", "error", "warn", "info", "debug", "trace"}
	case "hs.log.format":
		return []string{"text", "json"}
	}
	return nil
}

func managedLabel(key string) string {
	labels := map[string]string{
		"hs.metrics_listen_addr": "Metrics address",
		"hs.prefixes.v4":         "IPv4 prefix", "hs.prefixes.v6": "IPv6 prefix",
		"hs.prefixes.allocation": "Address allocation", "hs.disable_check_updates": "Disable update checks",
		"hs.log.level": "Log level", "hs.log.format": "Log format", "hs.logtail.enabled": "Client log collection",
		"hs.taildrop.enabled": "Taildrop", "hs.auto_update.enabled": "Client auto-update",
		"hs.node.expiry": "Default node expiry", "hs.node.ephemeral.inactivity_timeout": "Ephemeral inactivity timeout",
		"hs.ephemeral_node_inactivity_timeout": "Ephemeral inactivity timeout",
		"hs.derp.urls":                         "DERP map URLs", "hs.derp.auto_update_enabled": "DERP auto-update", "hs.derp.update_frequency": "DERP update interval",
		"hs.derp.server.enabled": "Embedded DERP", "hs.derp.server.region_id": "DERP region ID",
		"hs.derp.server.region_code": "DERP region code", "hs.derp.server.region_name": "DERP region name",
		"hs.derp.server.verify_clients": "DERP client verification", "hs.derp.server.stun_listen_addr": "STUN listen address",
		"hs.derp.server.automatically_add_embedded_derp_region": "Add embedded DERP region",
		"hs.derp.server.ipv4": "DERP public IPv4", "hs.derp.server.ipv6": "DERP public IPv6",
	}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}
