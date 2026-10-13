package config

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

const defaultPolicy = "{\n  \"acls\": [{\"action\": \"accept\", \"src\": [\"*\"], \"dst\": [\"*:*\"]}]\n}\n"

type policySource struct {
	document                *document
	mode, path, value, hash string
}

func (s *Store) readPolicy(database string) (*policySource, error) {
	d, err := readDocument(s.path)
	if err != nil {
		return nil, err
	}
	p := &policySource{document: d, mode: "file", value: defaultPolicy}
	if n := fieldNode(d.root.Content[0], "policy.mode"); n != nil {
		p.mode = n.Value
	}
	if n := fieldNode(d.root.Content[0], "policy.path"); n != nil {
		p.path = n.Value
	}
	switch p.mode {
	case "database":
		p.value = database
	case "file":
		if p.path != "" {
			if !filepath.IsAbs(p.path) {
				return nil, protocol.NewError(protocol.CodeClientConfig, "policy file requires an absolute path")
			}
			data, err := readFile(p.path)
			if err != nil {
				return nil, err
			}
			p.value = string(data)
		}
	default:
		return nil, protocol.NewError(protocol.CodeUnsupported, "unknown Headscale policy mode")
	}
	p.hash = fingerprint([]byte(p.mode), []byte(p.path), []byte(p.value))
	return p, nil
}

func (s *Store) PolicyMode() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readPolicy("")
	if err != nil {
		return "", err
	}
	return p.mode, nil
}

func (s *Store) policy(ctx context.Context, database string) (*protocol.PolicySettings, *policySource, error) {
	p, err := s.readPolicy(database)
	if err != nil {
		return nil, nil, err
	}
	out := &protocol.PolicySettings{Policy: p.value, Mode: p.mode, Writable: p.mode == "database" || writable(s.path, s.managedDir) && (p.path == "" || writable(p.path, s.managedDir))}
	var source, value string
	err = s.db.QueryRowContext(ctx, "SELECT source,value FROM policy_draft WHERE id=1").Scan(&source, &value)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	if err == nil {
		if source != p.hash {
			return nil, nil, protocol.NewError(protocol.CodeConflict, "Headscale policy changed; discard the draft and refresh")
		}
		out.Policy = value
		out.Pending = true
	}
	out.Revision = fingerprint([]byte(p.hash), []byte(value))
	return out, p, nil
}

func (s *Store) Policy(ctx context.Context, database string) (*protocol.PolicySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, _, err := s.policy(ctx, database)
	return out, err
}

func (s *Store) SavePolicy(ctx context.Context, req protocol.PolicyUpdate, database string) (*protocol.PolicySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, p, err := s.policy(ctx, database)
	if err != nil {
		return nil, err
	}
	if req.Revision != out.Revision {
		return nil, protocol.NewError(protocol.CodeConflict, "policy changed; refresh before saving")
	}
	if !out.Writable {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "policy is read-only; authorize through sudo")
	}
	if req.Policy == p.value {
		_, err = s.db.ExecContext(ctx, "DELETE FROM policy_draft WHERE id=1")
	} else {
		_, err = s.db.ExecContext(ctx, `INSERT INTO policy_draft(id,source,value) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET source=excluded.source,value=excluded.value`, p.hash, req.Policy)
	}
	if err != nil {
		return nil, err
	}
	out, _, err = s.policy(ctx, database)
	return out, err
}

func (s *Store) DiscardPolicy(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, "DELETE FROM policy_draft WHERE id=1")
	return err
}
