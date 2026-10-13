package tui

import (
	"encoding/json"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
)

func (m *Model) openResourceForm(a resourceAction) {
	if a.kind == "provider.edit" {
		m.providerForm(a.id)
		return
	}
	if strings.HasPrefix(a.kind, "account.") {
		m.accountForm(a.kind, a.id)
		return
	}
	f := &objectForm{title: a.label, target: a.target, help: a.help, key: a.kind}
	field := policyedit.Field{Name: "value", Label: "Name", Values: []string{a.value}}
	switch a.kind {
	case "node.tags":
		field.Label, field.Multi, field.Values = "Tags", true, splitValues(a.value)
		f.help = "Add one tag per item. An empty list clears all tags."
	case "node.routes":
		field.Label, field.Multi, field.Values = "Approved routes", true, splitValues(a.value)
		f.help = "Add one CIDR prefix per item. An empty list clears all approvals."
	case "node.owner":
		field.Name, field.Label = "owner", "Owner"
		for _, u := range m.users {
			field.Choices = append(field.Choices, u.ID)
		}
		f.help = "Choose the user who owns this device."
	}
	f.fields = []policyedit.Field{field}
	m.openObjectForm(f)
}

func (m *Model) openSettingForm(item *protocol.Setting) {
	f := &objectForm{title: settingLabel(item.Key), help: settingHelp(item.Key), key: item.Key}
	field := policyedit.Field{Name: "value", Label: "Value", Values: []string{item.Value}, Secret: item.Secret}
	if item.Key == "hs.derp.urls" || item.Key == "server.web.trusted_proxies" {
		field.Multi = true
		if err := json.Unmarshal([]byte(item.Value), &field.Values); err != nil {
			m.err = err
			return
		}
		field.Label = "URLs"
		f.help = "Add one value per item."
		if item.Key == "server.web.trusted_proxies" {
			field.Label = "Trusted proxies"
			f.help = settingHelp(item.Key)
		}
	}
	if strings.Contains(item.Key, "dns.") {
		kind, id, _ := strings.Cut(strings.TrimPrefix(item.Key, "dns."), ":")
		if kind == "add" {
			kind = id
			field.Values = []string{""}
		}
		switch kind {
		case "split", "record":
			name, value, _ := strings.Cut(item.Value, "=")
			if strings.HasPrefix(item.Key, "dns.add:") {
				name, value = "", ""
			}
			field.Name, field.Label, field.Values = "name", "Domain", []string{strings.TrimSpace(name)}
			second := policyedit.Field{Name: "value", Label: "Nameservers", Multi: true, Values: splitValues(value)}
			f.help = "Enter the domain and add one nameserver per item."
			if kind == "record" {
				second.Label, second.Multi, second.Values = "IP address", false, []string{strings.TrimSpace(value)}
				f.help = "Enter the record name and its IPv4 or IPv6 address."
			}
			f.fields = []policyedit.Field{field, second}
		case "global":
			field.Label = "Nameserver"
		case "search":
			field.Label = "Domain"
		}
	}
	if len(f.fields) == 0 {
		f.fields = []policyedit.Field{field}
	}
	m.openObjectForm(f)
}
