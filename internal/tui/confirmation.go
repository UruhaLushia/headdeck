package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type resourceConfirmation struct {
	action resourceAction
	button int
}

func (m *Model) openConfirmation(action resourceAction) {
	m.menu, m.resource = nil, nil
	m.editKey, m.editValue = "", ""
	m.confirmation = &resourceConfirmation{action: action}
	m.focus = focusContent
	m.err, m.notice = nil, ""
	if m.page == pageDNS {
		m.dnsGen++
	}
	if m.page == pageHeadscale {
		m.managedGen++
		m.managedLoading = false
	}
	if m.page == pagePolicy {
		m.policyGen++
		m.policyLoading = false
	}
}

func (m *Model) confirmationFrame() selectionFrame {
	p := m.confirmation
	w := min(54, max(24, m.width-6))
	inner := w - 6
	prompt := "Confirm this operation?"
	if strings.HasSuffix(p.action.kind, ".delete") {
		prompt = "Delete this item?"
	}
	lines := []string{titleStyle.Render(m.tr(p.action.label)), "",
		dimStyle.Width(inner).Background(surface).Render(m.tr(prompt)),
		titleStyle.Width(inner).MaxHeight(3).Background(surface).Render(p.action.target), ""}
	if p.action.help != "" {
		lines = append(lines, dimStyle.Width(inner).Background(surface).Render(m.tr(p.action.help)), "")
	}
	if m.err != nil {
		lines = append(lines, errStyle.Width(inner).MaxHeight(2).Background(surface).Render(m.formError()), "")
	}
	if m.resourceBusy {
		lines = append(lines, warnStyle.Render(m.tr("Saving…")), "")
	}
	buttonRow := lipgloss.Height(strings.Join(lines, "\n")) + 2
	var buttons []string
	for i, label := range []string{"Confirm", "Cancel"} {
		style := popupButtonStyle
		if p.button == i {
			style = style.Bold(true).Foreground(accent).Underline(true)
		}
		buttons = append(buttons, style.Render(m.tr(label)))
	}
	gap := lipgloss.NewStyle().Background(surface).Render("  ")
	lines = append(lines, strings.Join(buttons, gap), dimStyle.Render("←/→ · Enter · Esc"))
	view := panelStyle.Padding(1, 2).Width(w - 2).Background(surface).BorderForeground(accent).Render(strings.Join(lines, "\n"))
	w, h := lipgloss.Width(view), lipgloss.Height(view)
	x, y := max(0, (m.width-w)/2), max(0, (m.height-h)/2)
	return selectionFrame{x: x, y: y, width: w, height: h, buttonY: y + buttonRow, view: view}
}

func (m *Model) confirmResource() tea.Cmd {
	if m.confirmation.button == 1 {
		m.clearEditor()
		m.err = nil
		return nil
	}
	a := m.confirmation.action
	m.resource = &a
	m.editValue = a.confirm
	return m.saveResource()
}

func (m *Model) confirmationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearEditor()
		m.err = nil
	case "tab", "shift+tab":
		m.confirmation.button = 1 - m.confirmation.button
	case "left", "up", "home":
		m.confirmation.button = 0
	case "right", "down", "end":
		m.confirmation.button = 1
	case "enter", " ":
		return m, m.confirmResource()
	}
	return m, nil
}

func (m *Model) confirmationMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	f := m.confirmationFrame()
	inside := msg.X >= f.x && msg.X < f.x+f.width && msg.Y >= f.y && msg.Y < f.y+f.height
	if msg.Button == tea.MouseButtonRight || msg.Button == tea.MouseButtonLeft && !inside {
		m.clearEditor()
		m.err = nil
		return m, nil
	}
	if msg.Button != tea.MouseButtonLeft || msg.Y != f.buttonY {
		return m, nil
	}
	x := f.x + 3
	for i, label := range []string{"Confirm", "Cancel"} {
		width := lipgloss.Width(popupButtonStyle.Render(m.tr(label)))
		if msg.X >= x && msg.X < x+width {
			m.confirmation.button = i
			return m, m.confirmResource()
		}
		x += width + 2
	}
	return m, nil
}
