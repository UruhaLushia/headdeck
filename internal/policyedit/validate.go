package policyedit

import (
	"encoding/json"
	"fmt"
)

func (d *Document) validateShape() error {
	if raw := d.root["autoApprovers"]; len(raw) > 0 && string(raw) != "null" {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return fmt.Errorf("autoApprovers must be an object")
		}
	}
	for _, section := range Sections {
		raw := d.section(section)
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if listSection(section) {
			var items []map[string]json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				return fmt.Errorf("%s must be a list of objects", section)
			}
			for _, item := range items {
				if item == nil {
					return fmt.Errorf("%s contains an invalid item", section)
				}
				for _, name := range []string{"src", "dst", "users", "ip"} {
					if value := item[name]; len(value) > 0 {
						var values []string
						if err := json.Unmarshal(value, &values); err != nil {
							return fmt.Errorf("%s.%s must be a list of strings", section, name)
						}
					}
				}
				for _, name := range []string{"action", "proto", "checkPeriod"} {
					if value := item[name]; len(value) > 0 {
						var v string
						if err := json.Unmarshal(value, &v); err != nil {
							return fmt.Errorf("%s.%s must be a string", section, name)
						}
					}
				}
			}
		} else if section == "autoApprovers.exitNode" {
			var values []string
			if err := json.Unmarshal(raw, &values); err != nil {
				return fmt.Errorf("%s must be a list of strings", section)
			}
		} else {
			var values map[string]json.RawMessage
			if err := json.Unmarshal(raw, &values); err != nil {
				return fmt.Errorf("%s must be an object", section)
			}
			for _, value := range values {
				if section == "hosts" {
					var v string
					if err := json.Unmarshal(value, &v); err != nil {
						return fmt.Errorf("host aliases must be strings")
					}
				} else {
					var v []string
					if err := json.Unmarshal(value, &v); err != nil {
						return fmt.Errorf("%s entries must be lists of strings", section)
					}
				}
			}
		}
	}
	return nil
}
