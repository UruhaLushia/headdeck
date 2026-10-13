package tui

func (m *Model) isListPage() bool {
	if m.overviewNodes() {
		return true
	}
	switch m.page {
	case pageNodes, pageUsers, pageSettings, pageDNS, pageHeadscale, pagePolicy, pageKeys, pageAccounts, pageProviders:
		return true
	}
	return false
}

func (m *Model) navigationWindow() (int, int) {
	selected := 0
	for i, p := range navigationPages {
		if p == m.page || p == pageNodes && m.page == pageNodeDetail {
			selected = i
		}
	}
	if m.sidebarWidth() > 0 {
		if m.height >= 26 {
			return 0, len(navigationPages)
		}
		count := max(1, (m.height-8)/2)
		start := max(0, selected-count+1)
		return start, min(len(navigationPages), start+count)
	}
	width := m.layout().mainWidth
	start, used := selected, navigationWidth(m.tabLabel(selected, width))
	for start > 0 {
		next := navigationWidth(m.tabLabel(start-1, width)) + 1
		if used+next > width {
			break
		}
		used += next
		start--
	}
	end, used := start, 0
	for end < len(navigationPages) {
		next := navigationWidth(m.tabLabel(end, width))
		if end > start {
			next++
		}
		if used+next > width {
			break
		}
		used += next
		end++
	}
	return start, max(start+1, end)
}
