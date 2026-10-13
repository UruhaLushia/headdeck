package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func navigationWidth(label string) int { return lipgloss.Width(tabStyle.Render(label)) }

type navigationRow struct {
	index, row int
	heading    string
}

func (m *Model) navigationRows() []navigationRow {
	start, end := m.navigationWindow()
	if m.height < 26 {
		rows := make([]navigationRow, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, navigationRow{index: i, row: 1 + (i-start)*2})
		}
		return rows
	}
	rows := []navigationRow{{index: 0, row: 0}}
	row := 2
	for _, group := range []struct {
		name       string
		start, end int
	}{{"Network management", 1, 5}, {"Headscale", 5, 7}, {"System", 7, 10}} {
		rows = append(rows, navigationRow{heading: group.name, row: row})
		row++
		for i := group.start; i < group.end; i++ {
			rows = append(rows, navigationRow{index: i, row: row})
			row++
		}
		row++
	}
	return rows
}

func (m *Model) sidebar(width, height int) string {
	lines := make([]string, max(1, height-2))
	for _, entry := range m.navigationRows() {
		if entry.row >= len(lines) {
			continue
		}
		if entry.heading != "" {
			label := m.tr(entry.heading)
			lines[entry.row] = dimStyle.Render(" " + label + " " + strings.Repeat("─", max(0, width-4-lipgloss.Width(label))))
			continue
		}
		i := entry.index
		p := navigationPages[i]
		label := m.tr(pageNames[p])
		label = cell(label, width-5) + fmt.Sprint((i+1)%10)
		style := tabStyle
		if p == m.page || p == pageNodes && m.page == pageNodeDetail {
			style = activeTabStyle
			if m.focus == focusNavigation {
				style = focusedStyle
			}
		}
		if index, hit := m.navigationHit(m.mouseX, m.mouseY); hit && index == i {
			style = hoverStyle
		}
		lines[entry.row] = style.Render(label)
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(edge).Width(width - 2).Height(height - 2).Render(strings.Join(lines, "\n"))
}
