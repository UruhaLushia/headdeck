package tui

import (
	"fmt"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (m *Model) statusPanels() bool {
	return m.page == pageStatus && m.conn == connReady && m.status != nil && m.menu == nil
}

func (m *Model) overviewNodes() bool {
	return m.statusPanels() && m.sidebarWidth() > 0 && m.width-4-m.sidebarWidth() >= 92 && m.height >= 30
}

func nodeTableLine(width int, state, name, owner, address, seen string) string {
	ownerWidth, seenWidth := 16, 11
	nameWidth := max(12, (width-43)/2)
	addressWidth := max(15, width-nameWidth-ownerWidth-seenWidth-16)
	return cell(state, 8) + "  " + cell(name, nameWidth) + "  " + cell(owner, ownerWidth) + "  " + cell(address, addressWidth) + "  " + cell(seen, seenWidth)
}

func (m *Model) nodeTableHeader(width int) string {
	return nodeTableLine(width, m.tr("STATE"), m.tr("Name"), m.tr("OWNER"), m.tr("Addresses"), m.tr("LAST SEEN"))
}

func (m *Model) nodeTableRow(node protocol.Node, width int) string {
	state := okStyle.Render(m.tr("online"))
	if !node.Online {
		state = errStyle.Render(m.tr("offline"))
	}
	name := node.GivenName
	if name == "" {
		name = node.Name
	}
	address := "—"
	if len(node.IPAddresses) > 0 {
		address = node.IPAddresses[0]
	}
	return nodeTableLine(width, state, name, node.User.Name, address, shortTime(node.LastSeen))
}

func (m *Model) overviewNodeTable() string {
	width := m.contentWidth() - 4
	nodes := m.visibleNodes()
	start, end := m.listWindow(len(nodes), 14)
	rowWidth := width
	if len(nodes) > end-start {
		rowWidth -= 2
	}
	lines := []string{dimStyle.Render(m.nodeTableHeader(rowWidth)), dimStyle.Render(strings.Repeat("─", width))}
	for i := start; i < end; i++ {
		lines = append(lines, m.selectedRow(m.nodeTableRow(nodes[i], rowWidth), i))
	}
	if len(nodes) == 0 {
		lines = append(lines, dimStyle.Render(m.tr("No devices")))
	}
	lines = append(lines, dimStyle.Render(cell(fmt.Sprintf("%d–%d / %d", min(start+1, len(nodes)), end, len(nodes)), width)))
	title := m.tr("Nodes")
	if m.filter != "" || m.searching {
		title += " / " + m.filterDescription()
	}
	return dashboardPanel(title, strings.Join(lines, "\n"), m.contentWidth())
}
