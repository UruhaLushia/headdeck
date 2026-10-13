package tui

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

func (m *Model) accountForm(kind, id string) {
	if !m.can("accounts.write") {
		return
	}
	a := protocol.Account{Role: policy.RolePending}
	for _, item := range m.accounts {
		if item.ID == id {
			a = item
		}
	}
	f := &objectForm{key: kind, target: id, title: "Edit account"}
	add := func(name, label, value string, choices ...string) {
		f.fields = append(f.fields, policyedit.Field{Name: name, Label: label, Values: []string{value}, Choices: choices, Secret: name == "password"})
	}
	switch kind {
	case "account.password":
		f.title = "Set password"
		if a.HeadscaleUserID == "" {
			add("username", "Username", a.Username)
		}
		add("password", "New password", "")
	case "account.identity":
		f.title = "Bind identity"
		add("issuer", "Issuer", "")
		add("subject", "Subject", "")
	case "account.bind":
		f.title = "Headscale user"
		choices := []string{""}
		for _, u := range m.users {
			used := false
			for _, other := range m.accounts {
				if other.ID != id && other.HeadscaleUserID == u.ID {
					used = true
				}
			}
			if !used {
				choices = append(choices, u.ID)
			}
		}
		add("user", "Headscale user", a.HeadscaleUserID, choices...)
	default:
		if id == "" {
			f.title = "Create account"
			add("username", "Username", "")
			add("password", "Password", "")
		}
		add("label", "Display name", a.Label)
		add("role", "Role", string(a.Role), "pending", "member", "admin", "owner")
		enabled := "true"
		if a.Disabled {
			enabled = "false"
		}
		add("enabled", "Allow sign-in", enabled, "true", "false")
	}
	m.openObjectForm(f)
}
func (m *Model) saveAccountForm() tea.Cmd {
	f := m.objectForm
	kind, id := f.key, f.target
	v := map[string]string{}
	for _, field := range f.fields {
		if len(field.Values) > 0 {
			v[field.Name] = field.Values[0]
		}
	}
	a := protocol.Account{Role: policy.RolePending}
	for _, item := range m.accounts {
		if item.ID == id {
			a = item
		}
	}
	if kind == "account.create" || kind == "account.edit" {
		a.Label = v["label"]
		a.Role = policy.Role(v["role"])
		a.Disabled = v["enabled"] != "true"
		if id == "" {
			a.Username = v["username"]
		}
	}
	if kind == "account.bind" {
		a.HeadscaleUserID = v["user"]
	}
	m.resourceBusy = true
	m.accountsGen++
	gen := m.accountsGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 15*time.Second)
		defer cancel()
		var err error
		switch kind {
		case "account.password":
			username := a.Username
			if a.HeadscaleUserID == "" {
				username = v["username"]
			}
			err = m.client.SetAccountPassword(ctx, id, username, v["password"])
		case "account.identity":
			err = m.client.BindIdentity(ctx, id, protocol.Identity{Issuer: strings.TrimSpace(v["issuer"]), Subject: strings.TrimSpace(v["subject"])}, false)
		default:
			_, err = m.client.SaveAccountCredentials(ctx, id, protocol.AccountCredentials{Account: a, Password: v["password"]})
		}
		msg := accountsMsg{gen: gen, saved: true, err: err}
		if err == nil {
			out, e := m.client.Accounts(ctx)
			msg.err = e
			if out != nil {
				msg.accounts = out.Accounts
			}
		}
		return msg
	}
}
