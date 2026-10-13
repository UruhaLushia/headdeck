package policyedit

import (
	"encoding/json"
	"strings"
)

func (d *Document) Fields(item Item) []Field {
	values := d.item(item)
	var fields []Field
	add := func(name, label string, multi bool, fallback string, choices ...string) {
		f := Field{Name: name, Label: label, Multi: multi, Choices: choices}
		if multi {
			if err := json.Unmarshal(values[name], &f.Values); err != nil {
				f.Values = nil
			}
			if len(f.Values) == 0 {
				f.Values = []string{""}
			}
		} else {
			var value string
			if err := json.Unmarshal(values[name], &value); err != nil {
				value = ""
			}
			if item.Add && value == "" {
				value = fallback
			}
			f.Values = []string{value}
		}
		fields = append(fields, f)
	}
	if listSection(item.Section) {
		if item.Section == "ssh" {
			add("action", "Action", false, "accept", "accept", "check")
		}
		add("src", "Sources", true, "")
		add("dst", "Destinations", true, "")
		switch item.Section {
		case "acls":
			add("proto", "Protocol (optional)", false, "")
		case "grants":
			add("ip", "IP permissions", true, "")
		case "ssh":
			add("users", "SSH users", true, "")
			add("checkPeriod", "Check period (optional)", false, "")
		}
		return fields
	}
	if item.Section != "autoApprovers.exitNode" {
		fields = append(fields, Field{Name: "name", Label: "Name", Values: []string{item.Key}})
	}
	var raw json.RawMessage
	if item.Section == "autoApprovers.exitNode" {
		raw = d.section(item.Section)
	} else {
		var values map[string]json.RawMessage
		if err := json.Unmarshal(d.section(item.Section), &values); err != nil {
			values = nil
		}
		raw = values[item.Key]
	}
	f := Field{Name: "value", Multi: item.Section != "hosts"}
	f.Label = map[string]string{"hosts": "IP address or prefix", "groups": "Members", "tagOwners": "Owners", "autoApprovers.routes": "Approvers", "autoApprovers.exitNode": "Approvers"}[item.Section]
	if f.Multi {
		if err := json.Unmarshal(raw, &f.Values); err != nil {
			f.Values = nil
		}
	} else {
		f.Values = []string{display(raw)}
	}
	return append(fields, f)
}

func clean(values []string) []string {
	out := []string{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
