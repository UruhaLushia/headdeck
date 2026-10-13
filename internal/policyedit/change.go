package policyedit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

func (d *Document) Change(item Item, fields []Field, remove bool) (string, error) {
	var patches []patch
	path := itemPath(item)
	if remove {
		if item.Add {
			return "", fmt.Errorf("cannot delete a new item")
		}
		patches = append(patches, patch{Op: "remove", Path: path})
	} else {
		parent, _, nested := strings.Cut(item.Section, ".")
		if nested && (len(d.root[parent]) == 0 || string(d.root[parent]) == "null") {
			patches = append(patches, patch{Op: "add", Path: "/" + parent, Value: map[string]any{}})
		}
		if (len(d.section(item.Section)) == 0 || string(d.section(item.Section)) == "null") && item.Section != "autoApprovers.exitNode" {
			var container any = map[string]any{}
			if listSection(item.Section) {
				container = []any{}
			}
			patches = append(patches, patch{Op: "add", Path: "/" + strings.ReplaceAll(item.Section, ".", "/"), Value: container})
		}
		if listSection(item.Section) {
			if item.Add {
				path = "/" + item.Section + "/-"
				entry := map[string]any{}
				if item.Section == "acls" {
					entry["action"] = "accept"
				}
				patches = append(patches, patch{Op: "add", Path: path, Value: entry})
				var values []json.RawMessage
				if err := json.Unmarshal(d.section(item.Section), &values); err != nil {
					values = nil
				}
				path = fmt.Sprintf("/%s/%d", item.Section, len(values))
			}
			existing := d.item(item)
			allowed := d.Fields(item)
			for _, field := range fields {
				valid := false
				for _, a := range allowed {
					if a.Name == field.Name && a.Multi == field.Multi {
						valid = true
					}
				}
				if !valid {
					return "", fmt.Errorf("unsupported policy field: %s", field.Name)
				}
				var value any = clean(field.Values)
				if field.Multi && len(clean(field.Values)) == 0 && len(existing[field.Name]) == 0 {
					continue
				}
				if !field.Multi {
					value = strings.TrimSpace(strings.Join(field.Values, ""))
					if value == "" {
						if len(existing[field.Name]) > 0 {
							patches = append(patches, patch{Op: "remove", Path: path + "/" + pointer(field.Name)})
						}
						continue
					}
				}
				encoded, err := json.Marshal(value)
				if err != nil {
					return "", err
				}
				var compact bytes.Buffer
				if len(existing[field.Name]) > 0 {
					if err := json.Compact(&compact, existing[field.Name]); err != nil {
						return "", err
					}
				}
				if !bytes.Equal(compact.Bytes(), encoded) {
					patches = append(patches, patch{Op: "add", Path: path + "/" + pointer(field.Name), Value: value})
				}
			}
		} else {
			var name string
			var value any
			for _, field := range fields {
				switch field.Name {
				case "name":
					name = strings.TrimSpace(strings.Join(field.Values, ""))
				case "value":
					value = clean(field.Values)
					if item.Section == "hosts" {
						value = strings.TrimSpace(strings.Join(field.Values, ""))
					}
				default:
					return "", fmt.Errorf("unsupported policy field: %s", field.Name)
				}
			}
			if item.Section != "autoApprovers.exitNode" {
				if name == "" {
					return "", fmt.Errorf("name is required")
				}
				var existing map[string]json.RawMessage
				if err := json.Unmarshal(d.section(item.Section), &existing); err != nil {
					existing = nil
				}
				if (item.Add || name != item.Key) && len(existing[name]) > 0 {
					return "", fmt.Errorf("name already exists")
				}
				if !item.Add && name != item.Key {
					patches = append(patches, patch{Op: "remove", Path: path})
				}
				path = "/" + strings.ReplaceAll(item.Section, ".", "/") + "/" + pointer(name)
			}
			patches = append(patches, patch{Op: "add", Path: path, Value: value})
		}
	}
	if len(patches) == 0 {
		return string(d.ast.Pack()), nil
	}
	encoded, err := json.Marshal(patches)
	if err != nil {
		return "", err
	}
	ast := d.ast.Clone()
	if err := ast.Patch(encoded); err != nil {
		return "", err
	}
	return string(ast.Pack()), nil
}
