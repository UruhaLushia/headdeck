package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type settingSelection struct {
	key, value string
	choices    []string
	cursor     int
	offset     int
	button     int
}

type selectionFrame struct {
	x, y, width, height      int
	choiceY, buttonY         int
	choiceStart, choiceCount int
	view                     string
}

func (m *Model) settingPopupFrame() selectionFrame {
	p := m.settingPopup
	w := min(54, max(24, m.width-6))
	inner := w - 6
	count := max(1, m.height-12)
	p.offset = max(0, min(p.offset, len(p.choices)-count))
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+count {
		p.offset = p.cursor - count + 1
	}
	end := min(len(p.choices), p.offset+count)
	help := m.tr("Choose an option")
	if len(p.choices) > count {
		help += fmt.Sprintf(" · %d–%d/%d", p.offset+1, end, len(p.choices))
	}
	lines := []string{titleStyle.Render(cell(m.tr(strings.TrimPrefix(settingLabel(p.key), "object:")), inner)),
		dimStyle.Render(cell(help, inner)), ""}
	for i := p.offset; i < end; i++ {
		value := p.choices[i]
		mark := "  "
		if value == p.value {
			mark = "✓ "
		}
		style := dimStyle
		if i == p.cursor {
			style = selStyle
		}
		lines = append(lines, style.Render(cell(mark+m.popupChoiceLabel(value), inner)))
	}
	lines = append(lines, "")
	if m.err != nil {
		lines = append(lines, errStyle.Render(cell(m.formError(), inner)))
	} else if m.resourceBusy {
		lines = append(lines, warnStyle.Render(m.tr("Saving…")))
	}
	buttonRow := len(lines) + 2
	var buttons []string
	for i, label := range []string{"Confirm", "Cancel"} {
		style := popupButtonStyle
		if p.button == i {
			style = style.Bold(true).Foreground(accent).Underline(true)
		}
		buttons = append(buttons, style.Render(m.tr(label)))
	}
	gap := lipgloss.NewStyle().Background(surface).Render("  ")
	lines = append(lines, strings.Join(buttons, gap), dimStyle.Render("↑/↓ · Enter · Esc"))
	view := panelStyle.Padding(1, 2).Width(w - 2).Background(surface).
		BorderForeground(accent).Render(strings.Join(lines, "\n"))
	w, h := lipgloss.Width(view), lipgloss.Height(view)
	x, y := max(0, (m.width-w)/2), max(0, (m.height-h)/2)
	return selectionFrame{x: x, y: y, width: w, height: h, choiceY: y + 5,
		buttonY: y + buttonRow, view: view, choiceStart: p.offset, choiceCount: end - p.offset}
}

func (m *Model) overlaySettingPopup(background string) string {
	return m.overlayPopup(background, m.settingPopupFrame())
}

func (m *Model) overlayPopup(background string, f selectionFrame) string {
	base, popup := strings.Split(background, "\n"), strings.Split(f.view, "\n")
	for i, line := range popup {
		y := f.y + i
		if y >= len(base) {
			break
		}
		row := base[y] + strings.Repeat(" ", max(0, m.width-ansi.StringWidth(base[y])))
		left := ansi.Cut(row, 0, f.x)
		left += strings.Repeat(" ", max(0, f.x-ansi.StringWidth(left)))
		start, rightWidth := f.x+f.width, max(0, m.width-f.x-f.width)
		right := ansi.Cut(row, start, m.width)
		for ansi.StringWidth(right) > rightWidth {
			start++
			right = ansi.Cut(row, start, m.width)
		}
		right = strings.Repeat(" ", rightWidth-ansi.StringWidth(right)) + right
		base[y] = left + line + right
	}
	return strings.Join(base, "\n")
}

func (m *Model) cancelSettingPopup() {
	m.settingPopup = nil
	m.err = nil
	m.focus = focusContent
}

func (m *Model) confirmSettingPopup() tea.Cmd {
	p := m.settingPopup
	value := p.choices[p.cursor]
	if m.objectForm != nil && strings.HasPrefix(p.key, "object:") {
		f := m.objectForm
		c := f.controls()[f.cursor]
		f.fields[c.field].Values[c.entry] = value
		m.cancelSettingPopup()
		return nil
	}
	if value == p.value {
		m.cancelSettingPopup()
		return nil
	}
	return m.updateSetting(p.key, value)
}

func (m *Model) settingPopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.settingPopup
	switch msg.String() {
	case "esc":
		m.cancelSettingPopup()
	case "tab":
		p.button = (p.button+2)%3 - 1
	case "shift+tab":
		p.button = (p.button+3)%3 - 1
	case "up", "k":
		p.button = -1
		p.cursor = max(0, p.cursor-1)
	case "down", "j":
		p.button = -1
		p.cursor = min(len(p.choices)-1, p.cursor+1)
	case "pgup", "pgdown":
		step := max(1, m.height-12)
		if msg.String() == "pgup" {
			step = -step
		}
		p.button, p.cursor = -1, max(0, min(len(p.choices)-1, p.cursor+step))
	case "left", "right":
		if p.button >= 0 {
			p.button = 1 - p.button
		}
	case "home":
		p.button, p.cursor = -1, 0
	case "end":
		p.button, p.cursor = -1, len(p.choices)-1
	case "enter", " ":
		if p.button == 1 {
			m.cancelSettingPopup()
		} else {
			return m, m.confirmSettingPopup()
		}
	}
	return m, nil
}

func (m *Model) settingPopupMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	f := m.settingPopupFrame()
	inside := msg.X >= f.x && msg.X < f.x+f.width && msg.Y >= f.y && msg.Y < f.y+f.height
	if msg.Button == tea.MouseButtonRight || (msg.Button == tea.MouseButtonLeft && !inside) {
		m.cancelSettingPopup()
		return m, nil
	}
	p := m.settingPopup
	if !inside {
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		step := 1
		if msg.Button == tea.MouseButtonWheelUp {
			step = -1
		}
		p.button, p.cursor = -1, max(0, min(len(p.choices)-1, p.cursor+step))
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if index := msg.Y - f.choiceY; index >= 0 && index < f.choiceCount && msg.X >= f.x+3 && msg.X < f.x+f.width-3 {
		p.button, p.cursor = -1, f.choiceStart+index
	}
	if msg.Y == f.buttonY {
		x := f.x + 3
		for i, label := range []string{"Confirm", "Cancel"} {
			width := lipgloss.Width(popupButtonStyle.Render(m.tr(label)))
			if msg.X >= x && msg.X < x+width {
				if i == 0 {
					return m, m.confirmSettingPopup()
				}
				m.cancelSettingPopup()
			}
			x += width + 2
		}
	}
	return m, nil
}

func (m *Model) popupChoiceLabel(value string) string {
	if m.objectForm != nil {
		f := m.objectForm
		c := f.controls()[f.cursor]
		return m.formChoiceLabel(f.fields[c.field], value)
	}
	return m.choiceLabel(value)
}
