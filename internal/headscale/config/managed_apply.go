package config

import (
	"context"
	"errors"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"go.yaml.in/yaml/v3"
)

func applyManagedValues(d *document, values map[string]string) error {
	for key, value := range values {
		n := fieldNode(d.root.Content[0], key)
		parent := prefixParent(d.root.Content[0], key)
		if n == nil && parent == nil {
			return protocol.NewError(protocol.CodeConflict, "Headscale field is no longer available")
		}
		parsed, err := parseField(key, value)
		if err != nil {
			return err
		}
		if n == nil {
			parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimPrefix(key, "prefixes.")}, parsed)
			continue
		}
		parsed.HeadComment, parsed.LineComment, parsed.FootComment = n.HeadComment, n.LineComment, n.FootComment
		*n = *parsed
	}
	return validateManaged(d)
}

func (s *Store) ApplyManaged(ctx context.Context, revision string) (*Applied, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, d, values, err := s.managed(ctx)
	if err != nil {
		return nil, err
	}
	if out.Revision != revision {
		return nil, protocol.NewError(protocol.CodeConflict, "Headscale settings changed; refresh before applying")
	}
	if !out.Pending || !out.Writable {
		return nil, protocol.NewError(protocol.CodeBadRequest, "no writable pending Headscale changes")
	}
	if err := applyManagedValues(d, values); err != nil {
		return nil, err
	}
	data, err := renderDocument(d)
	if err != nil {
		return nil, err
	}
	a := &Applied{store: s, before: map[string][]byte{s.path: d.files[s.path]}, RestartRequired: true, table: "managed_draft"}
	if err := s.writeFile(s.path, data); err != nil {
		return nil, err
	}
	updated, err := readDocument(s.path)
	if err != nil {
		return nil, errors.Join(err, a.restore())
	}
	a.hash = updated.hash
	if err := s.advanceDrafts(ctx, d.hash, a.hash); err != nil {
		return nil, errors.Join(err, a.restore())
	}
	return a, nil
}

func (s *Store) advanceDrafts(ctx context.Context, from, to string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"dns_draft", "managed_draft"} {
		if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET source=? WHERE source=?", to, from); err != nil {
			return err
		}
	}
	return tx.Commit()
}
