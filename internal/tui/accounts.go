package tui

import (
	"context"
	"encoding/json"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

type accountsMsg struct {
	accounts []protocol.Account
	err      error
	gen      int
	saved    bool
}
type accountSecurityMsg struct {
	security *protocol.AccountSecurity
	err      error
}

func (m *Model) loadAccounts() tea.Cmd {
	if !m.can("accounts.read") || m.accountsLoading {
		return nil
	}
	m.accountsLoading = true
	m.accountsGen++
	gen := m.accountsGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.Accounts(ctx)
		msg := accountsMsg{gen: gen, err: err}
		if out != nil {
			msg.accounts = out.Accounts
		}
		return msg
	}
}
func (m *Model) loadAccountSecurity() tea.Cmd {
	if !m.can("accounts.read") || m.cursor < 0 || m.cursor >= len(m.accounts) {
		return nil
	}
	id := m.accounts[m.cursor].ID
	m.resourceBusy = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.AccountSecurity(ctx, id)
		return accountSecurityMsg{out, err}
	}
}
func (m *Model) accountsView() string {
	lines := []string{m.section("Access accounts", ""), ""}
	if !m.can("accounts.read") {
		return strings.Join(append(lines, m.tr("Owner required")), "\n")
	}
	w := m.contentWidth()
	col := max(12, w/3)
	lines = append(lines, dimStyle.Render(cell(m.tr("Account"), col)+cell(m.tr("Username"), col)+m.tr("Role")))
	start, end := m.listWindow(len(m.accounts), 6)
	for i := start; i < end; i++ {
		a := m.accounts[i]
		state := m.tr(string(a.Role))
		if a.Disabled {
			state += " · " + m.tr("Disabled")
		}
		lines = append(lines, m.selectedRow(cell(a.Label, col)+cell(a.Username, col)+state, i))
	}
	if len(m.accounts) == 0 {
		lines = append(lines, dimStyle.Render(m.tr("No access accounts yet.")))
	}
	if m.err != nil {
		lines = append(lines, "", errStyle.Render(m.formError()))
	}
	return strings.Join(lines, "\n")
}
func (m *Model) accountMenu() {
	if m.accountSecurity == nil {
		return
	}
	a := m.accountSecurity.Account
	m.menuCursor = 0
	m.focus = focusContent
	add := func(kind, label, value, confirm string) {
		m.menu = append(m.menu, resourceAction{kind: kind, label: label, id: a.ID, target: a.Label, value: value, confirm: confirm})
	}
	m.menu = nil
	if !m.can("accounts.write") {
		return
	}
	add("account.edit", "Edit account", "", "")
	add("account.bind", "Headscale user", "", "")
	add("account.password", "Set password", "", "")
	add("account.identity", "Bind identity", "", "")
	for _, i := range m.accountSecurity.Identities {
		encoded, err := json.Marshal(i.Identity)
		if err != nil {
			m.err = err
			return
		}
		add("account.unbind", "Unlink account", string(encoded), a.Label)
		m.menu[len(m.menu)-1].target = i.Subject
	}
	for _, key := range m.accountSecurity.Passkeys {
		add("account.passkey.delete", "Delete Passkey", key.ID, a.Label)
		m.menu[len(m.menu)-1].target = key.Name
	}
	if m.accountSecurity.TOTP {
		add("account.totp.reset", "Reset 2FA", "", a.Label)
	}
	add("account.sessions.revoke", "Sign out everywhere", "", a.Label)
}
