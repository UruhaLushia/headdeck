package tui

import tea "github.com/charmbracelet/bubbletea"

type focusArea int

const (
	focusContent focusArea = iota
	focusNavigation
	focusActions
)

func (m *Model) focusKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	key := msg.String()
	if key == "tab" || key == "shift+tab" {
		areas := []focusArea{focusNavigation, focusContent, focusActions}
		if m.dialogOpen() {
			areas = []focusArea{focusContent, focusActions}
		}
		index := 0
		for i, area := range areas {
			if area == m.focus {
				index = i
			}
		}
		step := 1
		if key == "shift+tab" {
			step = -1
		}
		m.focus = areas[(index+step+len(areas))%len(areas)]
		m.actionCursor = 0
		return true, nil
	}
	if m.focus == focusActions {
		count := len(m.visibleActions())
		m.actionCursor = max(0, min(m.actionCursor, count-1))
		switch key {
		case "left", "h":
			m.actionCursor = max(0, m.actionCursor-1)
		case "right", "l":
			m.actionCursor = min(max(0, count-1), m.actionCursor+1)
		case "home":
			m.actionCursor = 0
		case "end":
			m.actionCursor = max(0, count-1)
		case "pgup", "pgdown":
			return true, nil
		case "up", "esc":
			m.focus = focusContent
		case "down":
			m.focus = focusNavigation
			if m.dialogOpen() {
				m.focus = focusContent
			}
		case "enter", " ":
			if count > 0 {
				action := m.visibleActions()[m.actionCursor]
				m.focus = focusContent
				_, cmd := m.press(action.key)
				return true, cmd
			}
		default:
			return m.dialogOpen(), nil
		}
		return true, nil
	}
	if m.dialogOpen() {
		return false, nil
	}
	if m.focus == focusNavigation {
		switch key {
		case "up", "k", "pgup":
			return true, m.switchPage(-1)
		case "down", "j", "pgdown":
			return true, m.switchPage(1)
		case "home", "end":
			index := 0
			if key == "end" {
				index = len(navigationPages) - 1
			}
			m.page = navigationPages[index]
			m.cursor, m.listOffset, m.filter = 0, 0, ""
			return true, m.pageLoad()
		case "left", "h":
			if m.sidebarWidth() == 0 {
				return true, m.switchPage(-1)
			}
			return true, nil
		case "right", "l":
			if m.sidebarWidth() == 0 {
				return true, m.switchPage(1)
			}
			m.focus = focusContent
			return true, nil
		case "enter", " ":
			m.focus = focusContent
			return true, nil
		}
	}
	if m.focus == focusContent && m.page != pageNodeDetail {
		switch key {
		case "left":
			m.focus = focusNavigation
			return true, nil
		case "right":
			return true, nil
		case "esc":
			if m.filter == "" {
				m.focus = focusNavigation
				return true, nil
			}
		}
	}
	return false, nil
}
