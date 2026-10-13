package tui

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
)

func TestObjectFormKeyboardAndMouse(t *testing.T) {
	m := testModel()
	m.width, m.height = 48, 24
	fields := []policyedit.Field{
		{Name: "action", Label: "Action", Values: []string{"accept"}, Choices: []string{"accept", "check"}},
		{Name: "src", Label: "Sources", Multi: true, Values: []string{"alice", "bob", "carol", "dave", "erin", "frank"}},
		{Name: "dst", Label: "Destinations", Multi: true, Values: []string{"*"}},
	}
	m.openObjectForm(&objectForm{title: "SSH rule", fields: fields})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.settingPopup == nil {
		t.Fatal("enum field did not open a picker")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.settingPopup != nil || m.objectForm.fields[0].Values[0] != "check" {
		t.Fatal("picker did not update the form")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("新的用户")})
	if m.objectForm.fields[1].Values[0] != "新的用户" {
		t.Fatal("text input or field focus is incorrect")
	}
	m.objectForm.cursor = len(m.objectForm.controls()) - 1
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.objectForm.fields[2].Values) != 2 {
		t.Fatal("add item control failed")
	}
	frame := m.objectFrame()
	if frame.y < 0 || frame.y+frame.height > m.height {
		t.Fatal("form overflowed the terminal")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.objectForm.cursor == len(m.objectForm.controls())-2 {
		t.Fatal("page navigation did not move form focus")
	}
	frame = m.objectFrame()
	cancel := frame.buttons[1]
	m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: cancel.x, Y: cancel.y})
	if m.dialogOpen() {
		t.Fatal("cancel button did not close the form")
	}
}

func TestKeyFormAndCompactNavigation(t *testing.T) {
	m := testModel()
	m.hello = &protocol.HelloResponse{Features: []string{"keys.write"}}
	m.users = []protocol.User{{ID: "1", Name: "alice"}}
	m.newKeyForm()
	if m.objectForm.fields[1].Name != "expiry" || len(m.objectForm.fields[1].Choices) != 0 {
		t.Fatal("key expiration must remain a free-form value")
	}
	m.width, m.height = 48, 24
	secret := "one-time-secret" + strings.Repeat("x", 60) + "tail-value"
	m.keyCreated(keyCreatedMsg{key: &protocol.PreAuthKey{Key: secret}})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.objectForm.fields[0].Values[0] != secret || !strings.Contains(m.View(), "tail-value") {
		t.Fatal("creation result must be read-only")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.objectForm != nil || strings.Contains(m.View(), "one-time-secret") {
		t.Fatal("dismissed key material remained in the UI")
	}
	choices := make([]string, 100)
	for i := range choices {
		choices[i] = strings.Repeat("u", i+1)
	}
	m.width, m.height = 48, 24
	m.openObjectForm(&objectForm{fields: []policyedit.Field{{Name: "owner", Label: "User", Choices: choices, Values: []string{choices[0]}}}})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnd})
	frame := m.settingPopupFrame()
	if frame.y+frame.height > m.height || frame.choiceStart == 0 {
		t.Fatal("long selector was not kept within the terminal")
	}
	m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: frame.x + 3, Y: frame.choiceY})
	if m.settingPopup.cursor != frame.choiceStart {
		t.Fatal("scrolled selector mouse hit used the wrong index")
	}
	m.clearEditor()
	for _, size := range [][2]int{{48, 24}, {80, 24}, {100, 22}} {
		m.width, m.height, m.page = size[0], size[1], pageKeys
		m.View()
		start, end := m.navigationWindow()
		selected := 0
		for i, p := range navigationPages {
			if p == pageKeys {
				selected = i
			}
		}
		if start > selected || end <= selected {
			t.Fatal("selected page is outside the navigation viewport")
		}
		for y := range m.height {
			for x := range m.width {
				if index, hit := m.navigationHit(x, y); hit && (index < start || index >= end) {
					t.Fatal("navigation hit disagrees with its viewport")
				}
			}
		}
	}
	m.page, m.width, m.height, m.cursor = pagePolicy, 100, 30, 0
	m.policy = &protocol.PolicySettings{Policy: `{"acls":[{"action":"accept","src":["*"],"dst":["*:*"]}]}`}
	l := m.layout()
	m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: l.contentX, Y: l.listY + 1})
	if m.cursor != 1 {
		t.Fatal("policy row did not respond to mouse selection")
	}
	m.requestCtx, m.cancelRequests = context.WithCancel(context.Background())
	m.quit()
	if m.requestCtx.Err() != context.Canceled {
		t.Fatal("quit did not cancel pending requests")
	}
}
