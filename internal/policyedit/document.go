package policyedit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

type Item struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Index   int    `json:"index"`
	Add     bool   `json:"add"`
}

type Field struct {
	Secret  bool     `json:"secret,omitempty"`
	Name    string   `json:"name"`
	Label   string   `json:"label"`
	Values  []string `json:"values"`
	Choices []string `json:"choices"`
	Multi   bool     `json:"multi"`
}

type Document struct {
	ast  hujson.Value
	root map[string]json.RawMessage
}

var Sections = []string{"acls", "grants", "groups", "tagOwners", "hosts", "autoApprovers.routes", "autoApprovers.exitNode", "ssh"}

func Parse(value string) (*Document, error) {
	ast, err := hujson.Parse([]byte(value))
	if err != nil {
		return nil, err
	}
	standard := ast.Clone()
	standard.Standardize()
	d := &Document{ast: ast}
	if err := json.Unmarshal(standard.Pack(), &d.root); err != nil {
		return nil, err
	}
	if d.root == nil {
		return nil, fmt.Errorf("policy must be an object")
	}
	if err := d.validateShape(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Document) section(name string) json.RawMessage {
	parent, child, nested := strings.Cut(name, ".")
	if !nested {
		return d.root[name]
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(d.root[parent], &values); err != nil {
		values = nil
	}
	return values[child]
}

func listSection(section string) bool {
	return section == "acls" || section == "grants" || section == "ssh"
}

func (d *Document) Items() []Item {
	var out []Item
	for _, section := range Sections {
		raw := d.section(section)
		if listSection(section) {
			var values []map[string]json.RawMessage
			if json.Unmarshal(raw, &values) == nil {
				for i, value := range values {
					out = append(out, Item{Section: section, Index: i, Title: fmt.Sprintf("%s #%d", Label(section), i+1), Summary: summary(value)})
				}
			}
		} else if section == "autoApprovers.exitNode" {
			if len(raw) > 0 && string(raw) != "null" {
				out = append(out, Item{Section: section, Title: Label(section), Summary: display(raw)})
			}
		} else {
			var values map[string]json.RawMessage
			if json.Unmarshal(raw, &values) == nil {
				keys := make([]string, 0, len(values))
				for key := range values {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					out = append(out, Item{Section: section, Key: key, Title: key, Summary: display(values[key])})
				}
			}
		}
		if section != "autoApprovers.exitNode" || len(raw) == 0 || string(raw) == "null" {
			out = append(out, Item{Section: section, Title: Label(section), Add: true})
		}
	}
	return out
}

func (d *Document) item(item Item) map[string]json.RawMessage {
	if item.Add {
		return map[string]json.RawMessage{}
	}
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(d.section(item.Section), &values); err != nil {
		values = nil
	}
	if item.Index >= 0 && item.Index < len(values) {
		return values[item.Index]
	}
	return map[string]json.RawMessage{}
}

func display(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var values []string
	if json.Unmarshal(raw, &values) == nil {
		return strings.Join(values, ", ")
	}
	return string(raw)
}

func summary(values map[string]json.RawMessage) string {
	return display(values["src"]) + " → " + display(values["dst"])
}

func pointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func itemPath(item Item) string {
	base := "/" + strings.ReplaceAll(item.Section, ".", "/")
	if listSection(item.Section) {
		return base + "/" + strconv.Itoa(item.Index)
	}
	if item.Section == "autoApprovers.exitNode" {
		return base
	}
	return base + "/" + pointer(item.Key)
}

func Label(section string) string {
	return map[string]string{"acls": "Access rules", "grants": "Grants", "groups": "Groups", "tagOwners": "Tag owners", "hosts": "Host aliases", "autoApprovers.routes": "Route approvers", "autoApprovers.exitNode": "Exit node approvers", "ssh": "SSH rules"}[section]
}
