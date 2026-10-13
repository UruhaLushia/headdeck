package tui

import (
	"os"
	"os/exec"
	"os/user"

	tea "github.com/charmbracelet/bubbletea"
)

type authorizedMsg struct{ err error }

func (m *Model) authorize() tea.Cmd {
	executable, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return authorizedMsg{err: err} }
	}
	account, err := user.Current()
	if err != nil {
		return func() tea.Msg { return authorizedMsg{err: err} }
	}
	name := executable
	args := []string{"--lang", string(m.locale), "--users-file", m.authorization, "auth", "add", account.Username}
	if os.Geteuid() != 0 {
		name = "sudo"
		args = append([]string{"--", executable}, args...)
	}
	process := exec.Command(name, args...)
	return tea.ExecProcess(process, func(err error) tea.Msg { return authorizedMsg{err: err} })
}
