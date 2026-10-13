package tui

import (
	"encoding/json"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

func applyCollectionDiff[T any](data json.RawMessage, items *[]T, revision, wanted *uint64, id func(T) string) bool {
	var diff protocol.CollectionDiff[T]
	if err := json.Unmarshal(data, &diff); err != nil || diff.Revision == 0 || diff.Revision <= diff.BaseRevision {
		return false
	}
	if *revision != 0 && diff.Revision <= *revision {
		return true
	}
	*wanted = max(*wanted, diff.Revision)
	if *revision == 0 || diff.BaseRevision != *revision {
		return false
	}
	*items = diff.Apply(*items, id)
	*revision = diff.Revision
	return true
}

func (m *Model) applyEventDiff(ev protocol.Event) (bool, tea.Cmd) {
	switch ev.Type {
	case protocol.EventDNSChanged:
		if m.page == pageDNS {
			return false, nil
		}
		return true, nil
	case protocol.EventPolicyChanged:
		if m.page == pagePolicy {
			return false, nil
		}
		return true, nil
	case protocol.EventConfigChanged:
		if m.page == pageHeadscale {
			return false, nil
		}
		return true, nil
	case protocol.EventAccountsChanged:
		if m.page == pageAccounts {
			return false, nil
		}
		return true, nil
	case protocol.EventProvidersChanged:
		if m.page == pageProviders {
			return false, nil
		}
		return true, nil
	case protocol.EventKeysChanged:
		if m.page == pageKeys || m.page == pageStatus {
			return false, nil
		}
		return true, nil
	case protocol.EventRoutesChanged:
		return true, nil
	case protocol.EventNodesChanged:
		if !applyCollectionDiff(ev.Data, &m.nodes, &m.nodesRevision, &m.nodesWantedRevision, func(n protocol.Node) string { return n.ID }) {
			return true, m.loadNodes()
		}
		m.updateNodeDetail()
		m.clampCursor()
		return true, nil
	case protocol.EventUsersChanged:
		if !applyCollectionDiff(ev.Data, &m.users, &m.usersRevision, &m.usersWantedRevision, func(u protocol.User) string { return u.ID }) {
			return true, m.loadUsers()
		}
		m.clampCursor()
		return true, nil
	case protocol.EventOperationChanged:
		var op protocol.Operation
		if err := json.Unmarshal(ev.Data, &op); err == nil && op.ID != "" {
			if m.serviceOperation != nil && m.serviceOperation.ID == op.ID {
				_, cmd := m.serviceOperationUpdated(serviceOperationMsg{id: op.ID, op: &op})
				return true, cmd
			}
			return true, nil
		}
	}
	return false, nil
}

func (m *Model) updateNodeDetail() {
	if m.detail == nil {
		return
	}
	m.nodeDetailFromCache(m.detail.ID)
}

func (m *Model) nodeDetailFromCache(id string) {
	for _, node := range m.nodes {
		if node.ID == id {
			m.detail = &node
			return
		}
	}
	m.detail = nil
	if m.page == pageNodeDetail {
		m.page = pageNodes
	}
}
