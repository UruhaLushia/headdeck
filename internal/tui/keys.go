package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

type keysMsg struct {
	keys []protocol.PreAuthKey
	err  error
	gen  int
}

func (m *Model) keysUpdated(msg keysMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.keysGen {
		return m, nil
	}
	m.keysLoading, m.err = false, msg.err
	if msg.err == nil {
		m.keys = msg.keys
		m.keysLoaded = true
		m.clampCursor()
	}
	return m, nil
}

func (m *Model) loadKeys() tea.Cmd {
	if m.keysLoading || m.dialogOpen() || m.resourceBusy {
		return nil
	}
	m.keysLoading = true
	m.keysGen++
	gen, c := m.keysGen, m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := c.PreAuthKeys(ctx, "")
		msg := keysMsg{gen: gen, err: err}
		if out != nil {
			msg.keys = out.Keys
		}
		return msg
	}
}

func (m *Model) keysView() string {
	lines := []string{m.section("Auth keys", "Register devices without interactive sign-in."), ""}
	if len(m.keys) == 0 {
		lines = append(lines, dimStyle.Render(m.tr("No auth keys yet.")))
	} else {
		w := m.contentWidth()
		labelWidth := max(8, w/3)
		header := cell(m.tr("User"), labelWidth) + "  " + cell(m.tr("Status"), labelWidth) + "  " + m.tr("Expires")
		if w < 50 {
			header = cell(m.tr("User"), max(8, w-14)) + "  " + m.tr("Status")
		}
		lines = append(lines, dimStyle.Render(header))
		start, end := m.listWindow(len(m.keys), 6)
		for i := start; i < end; i++ {
			key := m.keys[i]
			state := "Single use"
			if key.Reusable {
				state = "Reusable"
			}
			if key.Used && !key.Reusable {
				state = "Used"
			}
			if key.Expired {
				state = "Expired"
			}
			name := key.User.Name
			if name == "" {
				name = key.User.ID
			}
			label := fmt.Sprintf("%s · #%s", name, key.ID)
			line := cell(label, labelWidth) + "  " + cell(m.tr(state), labelWidth) + "  " + dashIfEmpty(key.Expiration)
			if w < 50 {
				line = cell(label, max(8, w-14)) + "  " + m.tr(state)
			}
			lines = append(lines, m.selectedRow(line, i))
		}
	}
	if m.err != nil {
		lines = append(lines, "", errStyle.Render(m.formError()))
	}
	if m.notice != "" {
		lines = append(lines, "", dimStyle.Render(m.tr(m.notice)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) keyMenu() {
	if !m.can("keys.write") || m.cursor < 0 || m.cursor >= len(m.keys) {
		return
	}
	key := m.keys[m.cursor]
	target := fmt.Sprintf("%s · #%s", key.User.Name, key.ID)
	if !key.Expired && (key.Reusable || !key.Used) {
		m.menu = append(m.menu, resourceAction{kind: "key.expire", label: "Expire key", id: key.ID, target: target, confirm: key.ID, help: "This key will no longer allow new devices to register."})
	}
	m.menu = append(m.menu, resourceAction{kind: "key.delete", label: "Delete key", id: key.ID, target: target, confirm: key.ID, help: "This cannot be undone."})
}
