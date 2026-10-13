package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	accent     = lipgloss.AdaptiveColor{Light: "#1479CE", Dark: "#57C5FF"}
	muted      = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}
	edge       = lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#344D60"}
	surface    = lipgloss.AdaptiveColor{Light: "#F1F5F9", Dark: "#17232D"}
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle   = lipgloss.NewStyle().Foreground(muted)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#BE123C", Dark: "#FB7185"})
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"})
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"})
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#E4F2FF"}).
			Background(lipgloss.AdaptiveColor{Light: "#237AC4", Dark: "#245F97"})
	panelStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(edge).Padding(1)
	tabStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(muted)
	activeTabStyle = tabStyle.Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#071827"}).
			Background(lipgloss.AdaptiveColor{Light: "#237AC4", Dark: "#3C9CF4"})
	buttonStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(muted).
			Background(lipgloss.AdaptiveColor{Light: "#F1F5F9", Dark: "#1E293B"})
	focusedStyle = buttonStyle.Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#082F49"}).Background(accent)
	hoverStyle   = buttonStyle.Foreground(accent).Bold(true).
			Background(lipgloss.AdaptiveColor{Light: "#E0E7FF", Dark: "#334155"})
	popupButtonStyle = buttonStyle.Background(surface)
)

func section(title, subtitle string) string {
	return titleStyle.Render(title) + "\n" + dimStyle.Render(subtitle)
}

func cell(value string, width int) string {
	value = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(value)
	value = ansi.Truncate(strings.TrimSpace(value), width, "…")
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func (m *Model) contentWidth() int {
	if m.statusPanels() {
		return m.layout().mainWidth
	}
	return m.layout().contentWidth
}

func (m *Model) listWindow(total, reserved int) (int, int) {
	if m.overviewNodes() {
		reserved = 14
		if op := m.serviceOperation; op != nil {
			reserved++
			if op.Error != nil {
				reserved++
			}
		}
		if m.status.Headscale.LastError != nil {
			reserved++
		}
	}
	count := max(1, m.height-10-reserved)
	start := max(0, min(m.listOffset, total-count))
	if m.cursor < start {
		start = m.cursor
	}
	if m.cursor >= start+count {
		start = m.cursor - count + 1
	}
	start = max(0, min(start, total-count))
	m.listOffset = start
	return start, min(total, start+count)
}

func (m *Model) selectedRow(line string, index int) string {
	width := m.contentWidth()
	if m.overviewNodes() {
		width -= 4
	}
	reserved := 6
	if m.page == pageSettings {
		reserved = 9
	}
	start, end := m.listWindow(m.rowCount(), reserved)
	track := ""
	if m.rowCount() > end-start {
		count := end - start
		thumb := max(1, count*count/m.rowCount())
		position := start * (count - thumb) / max(1, m.rowCount()-count)
		track = dimStyle.Render("│")
		if index-start >= position && index-start < position+thumb {
			track = titleStyle.Render("┃")
		}
		width -= 2
	}
	line = cell(line, width)
	if index == m.cursor {
		line = selStyle.Render(ansi.Strip(line))
	} else if l := m.layout(); m.mouseX >= l.contentX && m.mouseX < l.contentX+l.contentWidth && m.mouseY == l.listY+index-start {
		line = lipgloss.NewStyle().Background(surface).Render(line)
	}
	if track != "" {
		return line + " " + track
	}
	return line
}
