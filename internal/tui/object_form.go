package tui

import (
	"strings"

	"github.com/UruhaLushia/headdeck/internal/policyedit"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type objectForm struct {
	title, target, help, key string
	fields                   []policyedit.Field
	item                     *policyedit.Item
	cursor, caret, offset    int
	readOnly                 bool
}

type formControl struct {
	field, entry int
	add          bool
}
type formHit struct {
	control, y, x, width int
	remove               bool
}
type objectFrame struct {
	selectionFrame
	hits    []formHit
	buttons []formHit
}

func (f *objectForm) buttons() []string {
	if f.readOnly {
		return []string{"Done"}
	}
	return []string{"Save", "Cancel"}
}

func (f *objectForm) controls() []formControl {
	var out []formControl
	for i, field := range f.fields {
		for j := range field.Values {
			out = append(out, formControl{field: i, entry: j})
		}
		if field.Multi {
			out = append(out, formControl{field: i, add: true})
		}
	}
	return out
}

func (m *Model) openObjectForm(f *objectForm) {
	m.objectForm = f
	m.menu, m.settingPopup = nil, nil
	m.editKey = "object"
	m.focus = focusContent
	m.err, m.notice = nil, ""
	if controls := f.controls(); len(controls) > 0 && !controls[0].add {
		c := controls[0]
		f.caret = len([]rune(f.fields[c.field].Values[c.entry]))
	}
}

func (m *Model) objectFrame() objectFrame {
	f := m.objectForm
	w := min(72, max(20, m.width-6))
	inner := w - 6
	controls := f.controls()
	f.cursor = max(0, min(f.cursor, len(controls)+len(f.buttons())-1))
	var lines []string
	var hits []formHit
	control := 0
	activeRow := 0
	inputStyle := lipgloss.NewStyle().Foreground(muted).Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#243244"}).Padding(0, 1)
	for _, field := range f.fields {
		lines = append(lines, titleStyle.Background(surface).Render(m.tr(field.Label)))
		for _, value := range field.Values {
			style := inputStyle
			selected := f.cursor == control
			if len(field.Choices) > 0 {
				value = m.formChoiceLabel(field, value) + "  ▾"
			}
			if selected {
				style = style.Foreground(accent)
				activeRow = len(lines)
				if len(field.Choices) == 0 && !f.readOnly {
					inputWidth := inner - 2
					if field.Multi {
						inputWidth -= 4
					}
					value = inputLine(value, f.caret, max(1, inputWidth), field.Secret)
				}
			} else if field.Secret {
				value = strings.Repeat("•", len([]rune(value)))
			}
			width := inner
			if field.Multi {
				width -= 4
			}
			if f.readOnly {
				for _, part := range strings.Split(ansi.Hardwrap(value, max(1, width-2), true), "\n") {
					hits = append(hits, formHit{control: control, y: len(lines), width: width})
					lines = append(lines, style.Render(cell(part, max(1, width-2))))
				}
				control++
				continue
			}
			line := style.Render(cell(value, max(1, width-2)))
			hits = append(hits, formHit{control: control, y: len(lines), width: width})
			if field.Multi {
				line += dimStyle.Background(surface).Render("  × ")
				hits = append(hits, formHit{control: control, y: len(lines), x: width, width: 4, remove: true})
			}
			lines = append(lines, line)
			control++
		}
		if field.Multi {
			style := dimStyle.Background(surface)
			if f.cursor == control {
				style = titleStyle.Background(surface)
				activeRow = len(lines)
			}
			lines = append(lines, style.Render("+ "+m.tr("Add item")))
			hits = append(hits, formHit{control: control, y: len(lines) - 1, width: inner})
			control++
		}
		lines = append(lines, "")
	}
	status := m.tr(f.help)
	if m.err != nil {
		status = m.formError()
	}
	if m.resourceBusy {
		status = m.tr("Saving…")
	}
	statusStyle := dimStyle
	if m.err != nil {
		statusStyle = errStyle
	}
	statusLines := strings.Split(statusStyle.Background(surface).Width(inner).MaxHeight(2).Render(status), "\n")
	if status == "" {
		statusLines = nil
	}
	available := max(1, m.height-11-len(statusLines))
	if f.cursor < len(controls) {
		if activeRow < f.offset {
			f.offset = activeRow
		}
		if activeRow >= f.offset+available {
			f.offset = activeRow - available + 1
		}
	}
	f.offset = max(0, min(f.offset, max(0, len(lines)-available)))
	end := min(len(lines), f.offset+available)
	body := []string{titleStyle.Background(surface).Render(cell(m.tr(f.title), inner)), dimStyle.Background(surface).Render(cell(f.target, inner)), ""}
	body = append(body, lines[f.offset:end]...)
	if len(lines) > available {
		body = append(body, dimStyle.Background(surface).Render(m.tr("Scroll to see more fields")))
	}
	body = append(body, statusLines...)
	body = append(body, "")
	buttonRow := len(body) + 2
	var buttons []formHit
	var buttonViews []string
	x := 0
	for i, label := range f.buttons() {
		style := popupButtonStyle
		if i == 0 {
			style = style.Bold(true).Foreground(accent)
		}
		if f.cursor == len(controls)+i {
			style = style.Bold(true).Foreground(accent).Underline(true)
		}
		v := style.Render(m.tr(label))
		buttons = append(buttons, formHit{control: len(controls) + i, x: x, width: lipgloss.Width(v)})
		buttonViews = append(buttonViews, v)
		x += lipgloss.Width(v) + 2
	}
	body = append(body, strings.Join(buttonViews, lipgloss.NewStyle().Background(surface).Render("  ")))
	view := panelStyle.Padding(1, 2).Width(w - 2).Background(surface).BorderForeground(accent).Render(strings.Join(body, "\n"))
	w, h := lipgloss.Width(view), lipgloss.Height(view)
	frame := selectionFrame{x: max(0, (m.width-w)/2), y: max(0, (m.height-h)/2), width: w, height: h, view: view}
	frame.buttonY = frame.y + buttonRow
	var visible []formHit
	for _, hit := range hits {
		if hit.y < f.offset || hit.y >= end {
			continue
		}
		hit.y += frame.y + 5 - f.offset
		hit.x += frame.x + 3
		visible = append(visible, hit)
	}
	for i := range buttons {
		buttons[i].x += frame.x + 3
		buttons[i].y = frame.buttonY
	}
	return objectFrame{selectionFrame: frame, hits: visible, buttons: buttons}
}

func (m *Model) formChoiceLabel(field policyedit.Field, value string) string {
	if field.Name == "owner" || field.Name == "user" {
		if value == "" {
			return m.tr("Unlinked")
		}
		for _, u := range m.users {
			if u.ID == value {
				return u.Name
			}
		}
	}
	if field.Name == "role" {
		return m.tr(value)
	}
	return m.choiceLabel(value)
}
