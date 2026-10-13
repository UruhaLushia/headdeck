package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (m *Model) statusView() string {
	if m.status == nil {
		return m.section("Overview", "Connecting to your control plane…")
	}
	s := m.status
	health := "Not configured"
	if s.Headscale.Configured {
		health = "Unavailable"
		if s.Headscale.Reachable {
			health = "Available"
			if s.Headscale.LastError != nil {
				health = "Degraded"
			}
		}
	}
	online := 0
	for _, n := range m.nodes {
		if n.Online {
			online++
		}
	}
	daemonRows := [][2]string{
		{"Process state", okStyle.Render(m.tr(s.Server.State))},
		{"Version", dashIfEmpty(s.Server.Version)},
		{"Uptime", m.uptime(s.Server.Uptime)},
		{"Memory (RSS)", protocol.FormatMemory(s.Server.RSSBytes)},
		{"Access", m.tr(s.Principal.Role)},
	}
	hsRows := [][2]string{
		{"Service state", m.serviceControlText(s.ServiceControl)},
		{"Version", dashIfEmpty(s.Headscale.Version)},
		{"API", m.tr(health)},
		{"Uptime", "-"}, {"Memory (RSS)", "-"},
	}
	if s.Headscale.Reachable && s.Headscale.LastError == nil {
		hsRows[2][1] = okStyle.Render(hsRows[2][1])
	} else if s.Headscale.Reachable {
		hsRows[2][1] = warnStyle.Render(hsRows[2][1])
	} else {
		hsRows[2][1] = errStyle.Render(hsRows[2][1])
	}
	if state := s.ServiceControl.State; state != nil {
		if state.ActiveState == "active" {
			hsRows[0][1] = okStyle.Render(hsRows[0][1])
		} else {
			hsRows[0][1] = warnStyle.Render(hsRows[0][1])
		}
		hsRows[3][1] = dashIfEmpty(state.Uptime)
		if state.ActiveState == "active" {
			hsRows[3][1] = m.uptime(state.Uptime)
		}
		hsRows[4][1] = protocol.FormatMemory(state.RSSBytes)
	}
	routes := make(map[string]bool)
	for _, node := range m.nodes {
		for _, route := range node.ApprovedRoutes {
			routes[route] = true
		}
	}
	keys := "—"
	if m.keysLoaded {
		keys = fmt.Sprint(len(m.keys))
	}
	networkRows := [][2]string{
		{"Nodes", fmt.Sprint(len(m.nodes))},
		{"online", okStyle.Render(fmt.Sprint(online))},
		{"offline", warnStyle.Render(fmt.Sprint(len(m.nodes) - online))},
		{"Users", fmt.Sprint(len(m.users))},
		{"Approved routes", fmt.Sprint(len(routes))},
		{"Auth keys", keys},
	}
	var info string
	if m.contentWidth() >= 64 {
		width := (m.contentWidth() - 2) / 2
		if m.contentWidth() >= 92 && m.height >= 28 {
			width = (m.contentWidth() - 4) / 3
		}
		daemonBody := detailRows(m.translatedRows(daemonRows), width-4)
		hsBody := detailRows(m.translatedRows(hsRows), width-4)
		if m.contentWidth() >= 92 && m.height >= 28 {
			daemonBody += "\n"
			hsBody += "\n"
		}
		info = lipgloss.JoinHorizontal(lipgloss.Top,
			dashboardPanel("Headdeck Daemon", daemonBody, width), "  ",
			dashboardPanel("Headscale", hsBody, width))
		if m.contentWidth() >= 92 && m.height >= 28 {
			info = lipgloss.JoinHorizontal(lipgloss.Top, info, "  ", dashboardPanel(m.tr("Network overview"), detailRows(m.translatedRows(networkRows), width-4), m.contentWidth()-2*width-4))
		} else if m.height >= 32 {
			info += "\n\n" + dashboardPanel(m.tr("Network overview"), detailRows(m.translatedRows(networkRows), m.contentWidth()-4), m.contentWidth())
		}
	} else {
		if m.height < 40 {
			daemonRows = append(daemonRows[:1], daemonRows[2:4]...)
			hsRows = append(hsRows[:1], hsRows[3:5]...)
		}
		panel := dashboardPanel
		if m.height < 24 {
			panel = func(title, body string, _ int) string {
				return titleStyle.Render(title) + "\n" + body
			}
		}
		width := m.contentWidth()
		info = panel("Headdeck Daemon", detailRows(m.translatedRows(daemonRows), width-4), width) +
			"\n" + panel("Headscale", detailRows(m.translatedRows(hsRows), width-4), width)
		if m.height >= 30 {
			rows := [][2]string{networkRows[0], networkRows[1], networkRows[3]}
			info += "\n" + panel(m.tr("Network overview"), detailRows(m.translatedRows(rows), width-4), width)
		} else {
			info += "\n" + dimStyle.Render(m.tr("Nodes")) + " " + networkRows[0][1] + " · " + m.tr("online") + " " + networkRows[1][1]
		}
	}
	body := info
	if m.overviewNodes() {
		body += "\n\n" + m.overviewNodeTable()
	}
	if op := m.serviceOperation; op != nil {
		body += "\n" + titleStyle.Render(m.tr("Service operation")) + " · " + m.tr(string(op.State))
		if op.Error != nil {
			body += "\n" + errStyle.Render(m.tr(op.Error.Message))
		}
	}
	if s.Headscale.LastError != nil {
		body += "\n" + errStyle.Render(s.Headscale.LastError.Message)
	}
	return body
}

func (m *Model) uptime(value string) string {
	duration, err := time.ParseDuration(value)
	if err != nil || m.statusReceivedAt.IsZero() {
		return dashIfEmpty(value)
	}
	return (duration + max(0, time.Since(m.statusReceivedAt))).Round(time.Second).String()
}

func dashboardPanel(title, body string, width int) string {
	inner := max(1, width-4)
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(edge).
		Width(max(1, width-2)).Padding(0, 1).Render(titleStyle.Render(cell(title, inner)) + "\n" + dimStyle.Render(strings.Repeat("─", inner)) + "\n" + body)
}

func detailRows(rows [][2]string, width int) string {
	var b strings.Builder
	labelWidth := min(18, max(10, width/3))
	if width >= 34 {
		labelWidth = max(14, labelWidth)
	}
	for _, row := range rows {
		b.WriteString(dimStyle.Render(cell(row[0], labelWidth)))
		b.WriteString("  ")
		b.WriteString(cell(row[1], max(1, width-labelWidth-2)))
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (m *Model) serviceControlText(sc protocol.ServiceControl) string {
	if !sc.Supported {
		if sc.Message != "" {
			return m.tr("unavailable: %s", sc.Message)
		}
		return m.tr("unavailable")
	}
	if sc.State != nil {
		return m.tr(sc.State.ActiveState)
	}
	return m.tr("Unknown")
}
