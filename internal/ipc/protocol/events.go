package protocol

import "encoding/json"

type Event struct {
	ID         uint64          `json:"id"`
	Type       string          `json:"type"`
	Timestamp  string          `json:"timestamp"`
	ResourceID string          `json:"resource_id,omitempty"`
	Revision   uint64          `json:"revision"`
	Data       json.RawMessage `json:"data,omitempty"`
}

const (
	EventNodesChanged       = "nodes.changed"
	EventUsersChanged       = "users.changed"
	EventKeysChanged        = "keys.changed"
	EventRoutesChanged      = "routes.changed"
	EventOperationChanged   = "operations.changed"
	EventHeadscaleRecovered = "headscale.recovered"
	EventDNSChanged         = "dns.changed"
	EventPolicyChanged      = "policy.changed"
	EventConfigChanged      = "config.changed"
	EventAccountsChanged    = "accounts.changed"
	EventProvidersChanged   = "providers.changed"
)

type CollectionDiff[T any] struct {
	BaseRevision uint64   `json:"base_revision"`
	Revision     uint64   `json:"revision"`
	Upsert       []T      `json:"upsert,omitempty"`
	Removed      []string `json:"removed,omitempty"`
}

func (d CollectionDiff[T]) Apply(items []T, id func(T) string) []T {
	changed := make(map[string]T, len(d.Upsert))
	removed := make(map[string]bool, len(d.Removed))
	for _, item := range d.Upsert {
		changed[id(item)] = item
	}
	for _, key := range d.Removed {
		removed[key] = true
	}
	out := make([]T, 0, len(items)+len(d.Upsert))
	for _, item := range items {
		key := id(item)
		if removed[key] {
			continue
		}
		if value, ok := changed[key]; ok {
			item = value
			delete(changed, key)
		}
		out = append(out, item)
	}
	for _, item := range d.Upsert {
		if key := id(item); !removed[key] {
			if _, ok := changed[key]; ok {
				out = append(out, item)
				delete(changed, key)
			}
		}
	}
	return out
}
