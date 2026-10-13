package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.connect(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type tickMsg time.Time

func (m *Model) connect() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 5*time.Second)
		defer cancel()
		resp, err := c.Hello(ctx, protocol.HelloRequest{
			Client:        "hdkt-tui",
			ClientVersion: "0.1.0",
			ProtocolMin:   protocol.SupportedVersions.Min,
			ProtocolMax:   protocol.SupportedVersions.Max,
			Features:      protocol.AllFeatures,
		})
		return helloMsg{resp: resp, err: err}
	}
}

func (m *Model) loadStatus(force ...bool) tea.Cmd {
	refresh := len(force) > 0 && force[0]
	if m.statusLoading {
		m.statusRefreshPending = m.statusRefreshPending || refresh
		return nil
	}
	m.statusLoading = true
	m.statGen++
	gen := m.statGen
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		status, err := c.Status(ctx, refresh)
		return statusMsg{gen: gen, status: status, err: err}
	}
}

func (m *Model) loadNodes() tea.Cmd {
	if m.nodesLoading {
		return nil
	}
	m.nodesLoading = true
	m.nodesGen++
	gen := m.nodesGen
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		list, err := c.Nodes(ctx, "")
		if err != nil {
			return nodesMsg{gen: gen, err: err}
		}
		return nodesMsg{gen: gen, nodes: list.Nodes, revision: list.Revision}
	}
}

func (m *Model) loadUsers() tea.Cmd {
	if m.usersLoading {
		return nil
	}
	m.usersLoading = true
	m.usersGen++
	gen := m.usersGen
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		list, err := c.Users(ctx)
		if err != nil {
			return usersMsg{gen: gen, err: err}
		}
		return usersMsg{gen: gen, users: list.Users, revision: list.Revision}
	}
}

func (m *Model) loadDetail(id string) tea.Cmd {
	c := m.client
	revision := m.nodesRevision
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		node, err := c.Node(ctx, id)
		return nodeDetailMsg{id: id, node: node, err: err, revision: revision}
	}
}
