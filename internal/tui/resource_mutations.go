package tui

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

type resourceResult struct {
	action    resourceAction
	node      *protocol.Node
	operation *protocol.Operation
	err       error
}

func splitValues(value string) []string {
	out := []string{}
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func (m *Model) saveResource() tea.Cmd {
	a, value := *m.resource, strings.TrimSpace(m.editValue)
	if a.kind == "policy.delete" {
		i, err := strconv.Atoi(a.id)
		items := m.policyItems()
		if err != nil || i < 0 || i >= len(items) || m.policy.Revision != a.value {
			return nil
		}
		return m.savePolicyObject(items[i], nil, true)
	}
	if a.kind == "dns.delete" {
		return m.deleteDNS(a.id)
	}
	dnsRevision := ""
	if m.dns != nil {
		dnsRevision = m.dns.Revision
	}
	m.resourceBusy = true
	m.err = nil
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 15*time.Second)
		defer cancel()
		return m.runResourceAction(ctx, a, value, dnsRevision)
	}
}

func (m *Model) runResourceAction(ctx context.Context, a resourceAction, value, dnsRevision string) resourceResult {
	out := resourceResult{action: a}
	switch a.kind {
	case "provider.delete":
		out.err = m.client.DeleteLoginProvider(ctx, a.id)
	case "key.delete":
		out.err = m.client.DeletePreAuthKey(ctx, a.id)
	case "account.unbind":
		var identity protocol.Identity
		if err := json.Unmarshal([]byte(a.value), &identity); err != nil {
			out.err = err
			break
		}
		out.err = m.client.BindIdentity(ctx, a.id, identity, true)
	case "account.totp.reset", "account.passkey.delete", "account.sessions.revoke":
		out.err = m.client.AccountSecurityAction(ctx, a.id, protocol.AccountSecurityAction{Action: strings.TrimPrefix(a.kind, "account."), ID: a.value})
	case "key.expire":
		out.err = m.client.ExpirePreAuthKey(ctx, a.id)
	case "config.apply":
		out.operation, out.err = m.client.ApplyManagedConfig(ctx, a.value)
	case "config.discard":
		out.err = m.client.DiscardManagedConfig(ctx)
	case "policy.apply":
		out.operation, out.err = m.client.ApplyPolicy(ctx, a.value)
	case "policy.discard":
		out.err = m.client.DiscardPolicy(ctx)
	case "dns.apply":
		out.operation, out.err = m.client.ApplyDNS(ctx, dnsRevision)
	case "dns.discard":
		out.err = m.client.DiscardDNS(ctx)
	case "service.start", "service.stop", "service.restart", "service.reload":
		out.operation, out.err = m.client.HeadscaleAction(ctx, protocol.ServiceAction(strings.TrimPrefix(a.kind, "service.")), "TUI")
	case "node.rename":
		out.node, out.err = m.client.RenameNode(ctx, a.id, value)
	case "node.tags":
		out.node, out.err = m.client.SetNodeTags(ctx, a.id, splitValues(value))
	case "node.routes":
		out.node, out.err = m.client.SetNodeRoutes(ctx, a.id, splitValues(value))
	case "node.owner":
		out.node, out.err = m.client.MoveNode(ctx, a.id, value)
	case "node.expire":
		out.node, out.err = m.client.ExpireNode(ctx, a.id)
	case "node.delete":
		out.err = m.client.DeleteNode(ctx, a.id, a.confirm)
	case "user.create":
		_, out.err = m.client.CreateUser(ctx, protocol.CreateUserRequest{Name: value})
	case "user.rename":
		_, out.err = m.client.RenameUser(ctx, a.id, value)
	case "user.delete":
		out.err = m.client.DeleteUser(ctx, a.id, a.confirm)
	default:
		out.err = protocol.NewError(protocol.CodeUnsupported, "unknown resource action")
	}
	return out
}

func (m *Model) resourceUpdated(msg resourceResult) (tea.Model, tea.Cmd) {
	m.resourceBusy = false
	m.err = msg.err
	if msg.err != nil {
		return m, nil
	}
	m.clearEditor()
	if msg.operation != nil {
		m.serviceOperation = msg.operation
		m.notice = "Service operation submitted"
		return m, tea.Batch(m.loadStatus(), m.pollServiceOperation(msg.operation.ID))
	}
	m.notice = "Changes saved"
	if msg.action.kind == "provider.delete" {
		return m, m.loadProviders()
	}
	if strings.HasPrefix(msg.action.kind, "account.") {
		return m, m.loadAccounts()
	}
	if strings.HasPrefix(msg.action.kind, "key.") {
		return m, m.loadKeys()
	}
	if strings.HasPrefix(msg.action.kind, "config.") || strings.HasPrefix(msg.action.kind, "policy.") {
		return m, m.pageLoad()
	}
	if strings.HasPrefix(msg.action.kind, "dns.") {
		return m, m.loadDNS()
	}
	if msg.node != nil && m.page == pageNodeDetail {
		if m.streamConnected {
			m.updateNodeDetail()
		} else {
			m.detail = msg.node
		}
	}
	if msg.action.kind == "node.delete" {
		m.page = pageNodes
		m.detail = nil
	}
	if !m.streamConnected {
		return m, tea.Batch(m.loadNodes(), m.loadUsers())
	}
	return m, nil
}
