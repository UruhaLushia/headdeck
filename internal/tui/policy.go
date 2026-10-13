package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
	tea "github.com/charmbracelet/bubbletea"
)

type policyMsg struct {
	settings       *protocol.PolicySettings
	err            error
	gen            int
	saved, checked bool
}

func (m *Model) loadPolicy() tea.Cmd {
	if m.dialogOpen() || m.resourceBusy || m.policyLoading {
		return nil
	}
	m.policyGen++
	gen := m.policyGen
	m.policyLoading = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.Policy(ctx)
		return policyMsg{settings: out, err: err, gen: gen}
	}
}

func (m *Model) policyUpdated(msg policyMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.policyGen {
		return m, nil
	}
	m.policyLoading = false
	m.err = msg.err
	if msg.saved || msg.checked {
		m.resourceBusy = false
	}
	if msg.err == nil {
		if msg.checked {
			m.notice = "Policy is valid"
			return m, nil
		}
		m.policy = msg.settings
		m.clampCursor()
		if msg.saved {
			m.clearEditor()
			m.notice = "Changes saved"
		}
	}
	return m, nil
}

func (m *Model) policyItems() []policyedit.Item {
	if m.policy == nil {
		return nil
	}
	d, err := policyedit.Parse(m.policy.Policy)
	if err != nil {
		return nil
	}
	return d.Items()
}

func (m *Model) beginPolicyEdit() {
	items := m.policyItems()
	if m.policy == nil || !m.policy.Writable || !m.can("policy.write") || m.cursor >= len(items) {
		return
	}
	d, err := policyedit.Parse(m.policy.Policy)
	if err != nil {
		m.err = err
		return
	}
	item := items[m.cursor]
	m.policyGen++
	m.policyLoading = false
	title := m.tr("Edit") + " · " + m.tr(policyedit.Label(item.Section))
	if item.Add {
		title = m.tr("Add") + " · " + m.tr(policyedit.Label(item.Section))
	}
	m.openObjectForm(&objectForm{title: title, target: m.policyItemTitle(item), help: "Advanced fields are preserved when saving.", item: &item, fields: d.Fields(item)})
}

func (m *Model) savePolicyObject(item policyedit.Item, fields []policyedit.Field, remove bool) tea.Cmd {
	d, err := policyedit.Parse(m.policy.Policy)
	if err != nil {
		m.err = err
		return nil
	}
	value, err := d.Change(item, fields, remove)
	if err != nil {
		m.err = err
		return nil
	}
	revision := m.policy.Revision
	m.policyGen++
	gen := m.policyGen
	m.resourceBusy = true
	m.err = nil
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 15*time.Second)
		defer cancel()
		out, err := client.SavePolicy(ctx, protocol.PolicyUpdate{Policy: value, Revision: revision})
		return policyMsg{settings: out, err: err, gen: gen, saved: true}
	}
}

func (m *Model) policyView() string {
	view := m.section("ACL", "Access rules, groups, tags and SSH policy")
	if m.policy == nil {
		return view
	}
	d, err := policyedit.Parse(m.policy.Policy)
	if err != nil {
		return view + "\n\n" + errStyle.Render(err.Error())
	}
	items := d.Items()
	lines := []string{view, "", dimStyle.Render(cell(m.tr("ITEM"), min(28, m.contentWidth()/2)) + "  " + m.tr("VALUE"))}
	start, end := m.listWindow(len(items), 6)
	for i := start; i < end; i++ {
		item := items[i]
		title := m.policyItemTitle(item)
		if item.Add {
			title = "+ " + m.tr("Add") + " · " + m.tr(item.Title)
		}
		lines = append(lines, m.selectedRow(cell(title, min(28, m.contentWidth()/2))+"  "+item.Summary, i))
	}
	state := m.tr("Select an item to edit")
	if m.policy.Pending {
		state = m.tr("Pending policy · press a to apply")
	}
	if !m.policy.Writable {
		state = m.tr("Read-only · press s to authorize through sudo")
	}
	if m.notice != "" {
		state = m.tr(m.notice)
	}
	if m.err != nil {
		state = m.formError()
	}
	return strings.Join(lines, "\n") + "\n\n" + dimStyle.Render(state)
}

func (m *Model) policyItemTitle(item policyedit.Item) string {
	if item.Section == "acls" || item.Section == "grants" || item.Section == "ssh" {
		return fmt.Sprintf("%s #%d", m.tr(policyedit.Label(item.Section)), item.Index+1)
	}
	if item.Section == "autoApprovers.exitNode" {
		return m.tr(item.Title)
	}
	return item.Title
}
