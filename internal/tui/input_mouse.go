package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func inputCursor(text string, position, column, widthAvailable int, secret bool) int {
	value := []rune(text)
	cursor := max(0, min(position, len(value)))
	before := lipgloss.Width(string(value[:cursor]))
	if secret {
		before = cursor
	}
	offset := max(0, before-max(1, widthAvailable)+1)
	position = 0
	width := 0
	for i, r := range value {
		size := lipgloss.Width(string(r))
		if secret {
			size = 1
		}
		if width+size > column+offset {
			break
		}
		width += size
		position = i + 1
	}
	return position
}
