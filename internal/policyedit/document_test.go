package policyedit

import (
	"strings"
	"testing"
)

func TestStructuredChangesPreserveAdvancedPolicy(t *testing.T) {
	const original = `{
 // Keep this explanation.
 "grants": [{"src": ["group:admins"], "dst": ["*"], "ip": ["*"], "app": {"example.test/cap": [{}]}}],
 "groups": {"group:admins": ["alice@example.test"]},
 "tests": [{"src": "alice@example.test", "accept": ["example.test:443"]}],
}`
	d, err := Parse(original)
	if err != nil {
		t.Fatal(err)
	}
	item := Item{Section: "grants", Index: 0}
	fields := d.Fields(item)
	unchanged, err := d.Change(item, fields, false)
	if err != nil || unchanged != original {
		t.Fatal("unchanged form rewrote the policy", err)
	}
	fields[1].Values = []string{"tag:web"}
	changed, err := d.Change(item, fields, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Keep this explanation.", `"app": {"example.test/cap": [{}]}`, `"tests": [{"src": "alice@example.test", "accept": ["example.test:443"]}]`, `"tag:web"`} {
		if !strings.Contains(changed, expected) {
			t.Fatalf("lost preserved field or comment: %s", changed)
		}
	}
	d, err = Parse(changed)
	if err != nil {
		t.Fatal(err)
	}
	changed, err = d.Change(Item{Section: "autoApprovers.routes", Add: true}, []Field{{Name: "name", Values: []string{"10.0.0.0/8"}}, {Name: "value", Multi: true, Values: []string{"group:admins"}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	d, err = Parse(changed)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range d.Items() {
		if item.Section == "autoApprovers.routes" && !item.Add {
			changed, err = d.Change(item, nil, true)
			if err != nil || strings.Contains(changed, "10.0.0.0/8") {
				t.Fatal("route approver deletion failed", err)
			}
			return
		}
	}
	t.Fatal("added route approver is missing")
}
