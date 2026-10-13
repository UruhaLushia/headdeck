package tui

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

type providersMsg struct {
	providers []protocol.LoginProvider
	err       error
	gen       int
	saved     bool
}

func (m *Model) loadProviders() tea.Cmd {
	if !m.can("accounts.read") || m.providersLoading {
		return nil
	}
	m.providersLoading = true
	m.providersGen++
	gen := m.providersGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 10*time.Second)
		defer cancel()
		out, err := m.client.LoginProviders(ctx)
		return providersMsg{providers: out, err: err, gen: gen}
	}
}
func (m *Model) providersView() string {
	lines := []string{m.section("Login providers", ""), ""}
	if !m.can("accounts.read") {
		return strings.Join(append(lines, m.tr("Owner required")), "\n")
	}
	col := max(12, m.contentWidth()/3)
	start, end := m.listWindow(len(m.providers), 6)
	for i := start; i < end; i++ {
		p := m.providers[i]
		state := "Disabled"
		if p.Enabled {
			state = "Enabled"
		}
		lines = append(lines, m.selectedRow(cell(p.Name, col)+cell(p.Kind, col)+m.tr(state), i))
	}
	if len(m.providers) == 0 {
		lines = append(lines, dimStyle.Render(m.tr("No login providers.")))
	}
	if m.err != nil {
		lines = append(lines, "", errStyle.Render(m.formError()))
	}
	return strings.Join(lines, "\n")
}
func (m *Model) providerMenu() {
	if !m.can("accounts.write") || m.cursor < 0 || m.cursor >= len(m.providers) {
		return
	}
	p := m.providers[m.cursor]
	m.menuCursor = 0
	m.focus = focusContent
	m.menu = []resourceAction{{kind: "provider.edit", label: "Edit provider", id: p.ID, target: p.Name}, {kind: "provider.delete", label: "Delete provider", id: p.ID, target: p.Name, confirm: p.Name}}
}
func (m *Model) providerForm(id string) {
	if !m.can("accounts.write") {
		return
	}
	p := protocol.LoginProvider{Kind: "oidc", AutoRegister: true}
	for _, item := range m.providers {
		if item.ID == id {
			p = item
		}
	}
	f := &objectForm{key: "provider.edit", target: id, title: "Login provider"}
	add := func(name, label, value string, choices ...string) {
		f.fields = append(f.fields, policyedit.Field{Name: name, Label: label, Values: []string{value}, Choices: choices, Secret: name == "client_secret"})
	}
	add("id", "ID", p.ID)
	add("name", "Name", p.Name)
	add("kind", "Type", p.Kind, "oidc", "github", "oauth")
	add("issuer", "Issuer", p.Issuer)
	add("client_id", "Client ID", p.ClientID)
	add("client_secret", "Client secret", "")
	add("auth_url", "Authorization URL", p.AuthURL)
	add("token_url", "Token URL", p.TokenURL)
	add("userinfo_url", "User info URL", p.UserinfoURL)
	add("scopes", "Scopes", strings.Join(p.Scopes, ", "))
	add("subject_claim", "Subject field", p.SubjectClaim)
	add("name_claim", "Name field", p.NameClaim)
	add("avatar_claim", "Avatar field", p.AvatarClaim)
	enabled, register := "false", "false"
	if p.Enabled {
		enabled = "true"
	}
	if p.AutoRegister {
		register = "true"
	}
	add("enabled", "Enabled", enabled, "true", "false")
	add("register", "Auto-register", register, "true", "false")
	m.openObjectForm(f)
}
func (m *Model) saveProviderForm() tea.Cmd {
	f := m.objectForm
	v := map[string]string{}
	for _, field := range f.fields {
		if len(field.Values) > 0 {
			v[field.Name] = strings.TrimSpace(field.Values[0])
		}
	}
	id := f.target
	if id == "" {
		id = v["id"]
	}
	p := protocol.LoginProvider{ID: id, Name: v["name"], Kind: v["kind"], Issuer: v["issuer"], ClientID: v["client_id"], ClientSecret: v["client_secret"], AuthURL: v["auth_url"], TokenURL: v["token_url"], UserinfoURL: v["userinfo_url"], Scopes: splitValues(v["scopes"]), SubjectClaim: v["subject_claim"], NameClaim: v["name_claim"], AvatarClaim: v["avatar_claim"], Enabled: v["enabled"] == "true", AutoRegister: v["register"] == "true"}
	m.providersGen++
	gen := m.providersGen
	m.resourceBusy = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.requestContext(), 15*time.Second)
		defer cancel()
		msg := providersMsg{gen: gen, saved: true, err: m.client.SaveLoginProvider(ctx, p)}
		if msg.err == nil {
			msg.providers, msg.err = m.client.LoginProviders(ctx)
		}
		return msg
	}
}
