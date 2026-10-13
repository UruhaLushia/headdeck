package config

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"go.yaml.in/yaml/v3"
)

func (s *Store) managed(ctx context.Context) (*protocol.ManagedConfig, *document, map[string]string, error) {
	d, err := readDocument(s.path)
	if err != nil {
		return nil, nil, nil, err
	}
	values := map[string]string{}
	var source, raw string
	err = s.db.QueryRowContext(ctx, "SELECT source,value FROM managed_draft WHERE id=1").Scan(&source, &raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, err
	}
	if err == nil {
		if source != d.hash {
			return nil, nil, nil, protocol.NewError(protocol.CodeConflict, "Headscale configuration changed; discard the draft and refresh")
		}
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, nil, nil, err
		}
	}
	out := &protocol.ManagedConfig{Items: []protocol.Setting{}, Writable: writable(s.path, s.managedDir), Pending: len(values) > 0, Revision: fingerprint([]byte(d.hash), []byte(raw))}
	for key := range managedFields {
		n := fieldNode(d.root.Content[0], key)
		if n == nil && prefixParent(d.root.Content[0], key) == nil {
			continue
		}
		value := ""
		if n != nil {
			value, err = fieldValue(n)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		if changed, exists := values[key]; exists {
			value = changed
		}
		out.Items = append(out.Items, protocol.Setting{Key: "hs." + key, Value: value})
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Key < out.Items[j].Key })
	return out, d, values, nil
}

func (s *Store) Managed(ctx context.Context) (*protocol.ManagedConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, _, _, err := s.managed(ctx)
	return out, err
}

func (s *Store) SaveManaged(ctx context.Context, req protocol.ManagedConfigUpdate) (*protocol.ManagedConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, d, values, err := s.managed(ctx)
	if err != nil {
		return nil, err
	}
	if !out.Writable {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "Headscale configuration is read-only; authorize through sudo")
	}
	if req.Revision != out.Revision {
		return nil, protocol.NewError(protocol.CodeConflict, "Headscale settings changed; refresh before saving")
	}
	for key, value := range req.Values {
		key = strings.TrimPrefix(key, "hs.")
		n := fieldNode(d.root.Content[0], key)
		if n == nil && prefixParent(d.root.Content[0], key) == nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "unknown or unavailable Headscale setting")
		}
		_, err := parseField(key, value)
		if err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "%s", err)
		}
		original := ""
		if n != nil {
			original, err = fieldValue(n)
			if err != nil {
				return nil, err
			}
		}
		if original == value {
			delete(values, key)
		} else {
			values[key] = value
		}
	}
	if err := applyManagedValues(d, values); err != nil {
		return nil, protocol.NewError(protocol.CodeBadRequest, "%s", err)
	}
	if len(values) == 0 {
		_, err = s.db.ExecContext(ctx, "DELETE FROM managed_draft WHERE id=1")
	} else {
		raw, marshalErr := json.Marshal(values)
		if marshalErr != nil {
			return nil, marshalErr
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO managed_draft(id,source,value) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET source=excluded.source,value=excluded.value`, d.hash, string(raw))
	}
	if err != nil {
		return nil, err
	}
	out, _, _, err = s.managed(ctx)
	return out, err
}

func (s *Store) DiscardManaged(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, "DELETE FROM managed_draft WHERE id=1")
	return err
}

func renderDocument(d *document) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&d.root); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func validateManaged(d *document) error {
	root := d.root.Content[0]
	if prefixes := fieldNode(root, "prefixes"); prefixes != nil {
		v4, v6 := child(prefixes, "v4"), child(prefixes, "v6")
		if (v4 == nil || v4.Value == "") && (v6 == nil || v6.Value == "") {
			return errors.New("at least one IP prefix must be configured")
		}
	}
	if n := fieldNode(root, "server_url"); n != nil {
		if err := Validate(d.config, n.Value); err != nil {
			return err
		}
	}
	if enabled := fieldNode(root, "derp.server.enabled"); enabled != nil && enabled.Value == "true" {
		server := fieldNode(root, "server_url")
		if server == nil || !strings.HasPrefix(server.Value, "https://") {
			return errors.New("embedded DERP requires an HTTPS server URL")
		}
		for _, key := range []string{"derp.server.stun_listen_addr", "derp.server.private_key_path"} {
			if n := fieldNode(root, key); n == nil || n.Value == "" {
				return errors.New("embedded DERP requires a STUN address and a private key path")
			}
		}
	}
	return nil
}
