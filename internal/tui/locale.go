package tui

import "fmt"

func (m *Model) tr(message string, args ...any) string { return m.locale.Text(message, args...) }

func (m *Model) section(title, subtitle string) string { return section(m.tr(title), m.tr(subtitle)) }

func (m *Model) tabLabel(index int, width int) string {
	name := pageNames[navigationPages[index]]
	if width < 100 {
		name = []string{"Home", "Nodes", "Users", "Keys", "DNS", "HS", "ACL", "Cfg", "Accounts", "Login"}[index]
	}
	return fmt.Sprintf("%d %s", (index+1)%10, m.tr(name))
}

func (m *Model) actionLabel(action uiAction) string { return m.tr(action.label) + " " + action.key }

func (m *Model) translatedRows(rows [][2]string) [][2]string {
	for i := range rows {
		rows[i][0] = m.tr(rows[i][0])
	}
	return rows
}
