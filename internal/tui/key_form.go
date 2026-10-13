package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
	tea "github.com/charmbracelet/bubbletea"
)

type keyCreatedMsg struct {
	key *protocol.PreAuthKey
	err error
}

func (m *Model) keyCreated(msg keyCreatedMsg) (tea.Model, tea.Cmd) {
	m.resourceBusy, m.err = false, msg.err
	if msg.err != nil {
		return m, nil
	}
	m.clearEditor()
	if msg.key != nil {
		m.openObjectForm(&objectForm{key: "key.created", title: "Your new auth key", help: "Copy this key now. It will only be shown once.", readOnly: true, fields: []policyedit.Field{{Name: "key", Label: "Key", Values: []string{msg.key.Key}}}})
	}
	return m, nil
}

func (m *Model) newKeyForm() {
	if !m.can("keys.write") {
		return
	}
	if len(m.users) == 0 {
		m.err = fmt.Errorf("create a user first")
		return
	}
	choices := make([]string, 0, len(m.users))
	for _, u := range m.users {
		choices = append(choices, u.ID)
	}
	m.openObjectForm(&objectForm{key: "key.create", title: "Create auth key", help: "Use a duration such as 24h or 90d, or a date and time.", fields: []policyedit.Field{
		{Name: "owner", Label: "User", Values: []string{choices[0]}, Choices: choices},
		{Name: "expiry", Label: "Expiration", Values: []string{"24h"}},
		{Name: "reusable", Label: "Reusable", Values: []string{"false"}, Choices: []string{"false", "true"}},
		{Name: "ephemeral", Label: "Ephemeral", Values: []string{"false"}, Choices: []string{"false", "true"}},
		{Name: "tags", Label: "Tags", Multi: true, Values: []string{}},
	}})
}

func (m *Model) createKey() tea.Cmd {
	values := make(map[string][]string)
	for _, field := range m.objectForm.fields {
		values[field.Name] = field.Values
	}
	request := protocol.CreatePreAuthKeyRequest{User: values["owner"][0], Expiration: values["expiry"][0], Reusable: values["reusable"][0] == "true", Ephemeral: values["ephemeral"][0] == "true"}
	for _, tag := range values["tags"] {
		if tag = strings.TrimSpace(tag); tag != "" {
			request.ACLTags = append(request.ACLTags, tag)
		}
	}
	m.resourceBusy, m.err = true, nil
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 15*time.Second)
		defer cancel()
		key, err := c.CreatePreAuthKey(ctx, request)
		return keyCreatedMsg{key: key, err: err}
	}
}
