package tui

type workspaceLayout struct {
	sidebar, mainX, mainWidth, contentX, contentWidth int
	listY, footerY                                    int
}

func (m *Model) layout() workspaceLayout {
	sidebar := m.sidebarWidth()
	l := workspaceLayout{
		sidebar: sidebar, mainX: 2 + sidebar, mainWidth: max(1, m.width-4-sidebar),
		contentX: 4 + sidebar, contentWidth: max(1, m.width-8-sidebar),
		listY: 10, footerY: m.height - 2,
	}
	if m.overviewNodes() {
		l.listY = 19
	}
	if category := m.settingsCategory(); category != nil && *category == "" && m.menu == nil {
		l.listY--
	}
	return l
}

func (m *Model) sidebarWidth() int {
	if m.width >= 96 && m.height >= 22 {
		return 22
	}
	return 0
}

func (m *Model) navigationHit(x, y int) (int, bool) {
	l := m.layout()
	start, end := m.navigationWindow()
	if l.sidebar > 0 {
		if x >= 3 && x < l.mainX-3 {
			for _, entry := range m.navigationRows() {
				if entry.heading == "" && y == 4+entry.row {
					return entry.index, true
				}
			}
		}
		return 0, false
	}
	if y != 3 {
		return 0, false
	}
	left := l.mainX
	for i := start; i < end; i++ {
		width := navigationWidth(m.tabLabel(i, l.mainWidth))
		if x >= left && x < left+width {
			return i, true
		}
		left += width + 1
	}
	return 0, false
}
