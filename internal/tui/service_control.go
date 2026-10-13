package tui

import (
	"context"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) openServiceMenu() {
	if !m.can("headscale.restart") || m.status == nil || !m.status.ServiceControl.Supported {
		return
	}
	if op := m.serviceOperation; op != nil && (op.State == protocol.OperationRunning || op.State == protocol.OperationPending) {
		m.notice = "A service operation is already running"
		return
	}
	control := m.status.ServiceControl
	for _, item := range []struct{ action, label string }{
		{"start", "Start Headscale"}, {"stop", "Stop Headscale"},
		{"restart", "Restart Headscale"}, {"reload", "Reload Headscale"},
	} {
		if item.action == "reload" && (control.State == nil || !control.State.CanReload) {
			continue
		}
		m.menu = append(m.menu, resourceAction{kind: "service." + item.action,
			label: item.label, target: control.Unit, confirm: control.Unit,
			help: "This changes the state of the Headscale service."})
	}
}

type serviceOperationMsg struct {
	id  string
	op  *protocol.Operation
	err error
}

func (m *Model) pollServiceOperation(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		op, err := m.client.Operation(ctx, id)
		return serviceOperationMsg{id: id, op: op, err: err}
	}
}

func (m *Model) serviceOperationUpdated(msg serviceOperationMsg) (tea.Model, tea.Cmd) {
	if m.serviceOperation == nil || m.serviceOperation.ID != msg.id {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	if m.serviceOperation.State == protocol.OperationSucceeded || m.serviceOperation.State == protocol.OperationFailed || m.serviceOperation.State == protocol.OperationTimeout {
		return m, nil
	}
	m.serviceOperation = msg.op
	switch msg.op.State {
	case protocol.OperationSucceeded:
		m.notice = "Service operation completed"
		if m.page == pageDNS {
			return m, tea.Batch(m.loadStatus(), m.loadDNS())
		}
		if m.page == pageHeadscale || m.page == pagePolicy {
			return m, tea.Batch(m.loadStatus(), m.pageLoad())
		}
		return m, m.loadStatus()
	case protocol.OperationFailed, protocol.OperationTimeout:
		m.err = msg.op.Error
		m.notice = "Service operation failed"
		return m, m.loadStatus()
	default:
		return m, nil
	}
}
