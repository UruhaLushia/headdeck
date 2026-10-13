package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"os/exec"
)

type dnsAuthorizedMsg struct{ err error }

func (m *Model) authorizeDNS() tea.Cmd {
	if !m.can("dns.write") && !m.can("headscale.config.write") && !m.can("policy.write") {
		return nil
	}
	if m.page == pageDNS && m.dns != nil && m.dns.Writable {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return dnsAuthorizedMsg{err: err} }
	}
	name := executable
	args := []string{"headscale", "config", "authorize"}
	if os.Geteuid() != 0 {
		name = "sudo"
		args = append([]string{"--", executable}, args...)
	}
	return tea.ExecProcess(exec.Command(name, args...), func(err error) tea.Msg { return dnsAuthorizedMsg{err: err} })
}
