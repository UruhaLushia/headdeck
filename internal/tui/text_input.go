package tui

import tea "github.com/charmbracelet/bubbletea"

func editText(text string, cursor int, msg tea.KeyMsg) (string, int) {
	value := []rune(text)
	cursor = max(0, min(cursor, len(value)))
	switch msg.String() {
	case "ctrl+u":
		return "", 0
	case "left":
		cursor = max(0, cursor-1)
	case "right":
		cursor = min(len(value), cursor+1)
	case "home", "ctrl+a":
		cursor = 0
	case "end", "ctrl+e":
		cursor = len(value)
	case "backspace":
		if cursor > 0 {
			value = append(value[:cursor-1], value[cursor:]...)
			cursor--
		}
	case "delete":
		if cursor < len(value) {
			value = append(value[:cursor], value[cursor+1:]...)
		}
	default:
		var input []rune
		if msg.Type == tea.KeyRunes {
			input = msg.Runes
		}
		if msg.Type == tea.KeySpace {
			input = []rune{' '}
		}
		clean := make([]rune, 0, len(input))
		for _, r := range input {
			if r >= 32 && r != 127 {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 && len(value)+len(clean) <= 65536 {
			next := make([]rune, 0, len(value)+len(clean))
			next = append(next, value[:cursor]...)
			next = append(next, clean...)
			value = append(next, value[cursor:]...)
			cursor += len(clean)
		}
	}
	return string(value), cursor
}
