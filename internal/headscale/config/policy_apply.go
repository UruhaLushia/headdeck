package config

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/securefile"
	"go.yaml.in/yaml/v3"
)

type PolicyApplied struct {
	RestartRequired                bool
	store                          *Store
	before                         map[string][]byte
	created                        string
	path, hash, configHash, source string
}

func (s *Store) ApplyPolicyFile(ctx context.Context, revision string) (*PolicyApplied, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, p, err := s.policy(ctx, "")
	if err != nil {
		return nil, err
	}
	if out.Revision != revision || !out.Pending {
		return nil, protocol.NewError(protocol.CodeConflict, "policy changed; refresh before applying")
	}
	if p.mode != "file" || !out.Writable {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "policy file is read-only")
	}
	a := &PolicyApplied{store: s, before: make(map[string][]byte), path: p.path, source: p.hash, RestartRequired: true}
	if p.path == "" {
		a.before[s.path] = p.document.files[s.path]
		a.RestartRequired = true
		a.path = filepath.Join(s.managedDir, "policy.hujson")
		if err := s.createPolicy(a.path, []byte(out.Policy)); err != nil {
			return nil, err
		}
		a.created = a.path
		root := p.document.root.Content[0]
		n := child(root, "policy")
		if n == nil {
			n = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "policy"}, n)
		}
		path := child(n, "path")
		if path == nil {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "path"}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: a.path})
		} else {
			path.Value = a.path
		}
		data, err := renderDocument(p.document)
		if err != nil {
			return nil, errors.Join(err, a.restore())
		}
		if err := s.writeFile(s.path, data); err != nil {
			return nil, errors.Join(err, a.restore())
		}
	} else {
		a.before[a.path] = []byte(p.value)
		if err := s.writeFile(a.path, []byte(out.Policy)); err != nil {
			return nil, err
		}
	}
	updated, err := s.readPolicy("")
	if err != nil {
		return nil, errors.Join(err, a.restore())
	}
	a.hash = updated.hash
	a.configHash = updated.document.hash
	if _, err := s.db.ExecContext(ctx, "UPDATE policy_draft SET source=? WHERE id=1", a.hash); err != nil {
		return nil, errors.Join(err, a.restore())
	}
	if err := s.advanceDrafts(ctx, p.document.hash, a.configHash); err != nil {
		_, resetErr := s.db.ExecContext(ctx, "UPDATE policy_draft SET source=? WHERE id=1", a.source)
		return nil, errors.Join(err, a.restore(), resetErr)
	}
	return a, nil
}

func (s *Store) createPolicy(path string, data []byte) error {
	if filepath.Dir(path) != s.managedDir {
		return fmt.Errorf("policy must be inside the managed configuration directory")
	}
	if err := securefile.Directory(s.managedDir); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.managedDir)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine Headscale configuration group")
	}
	temporary := ".policy-" + rand.Text()
	f, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	defer f.Close()
	if err := f.Chown(-1, int(stat.Gid)); err != nil {
		return err
	}
	if err := f.Chmod(0o640); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return root.Link(temporary, filepath.Base(path))
}

func (a *PolicyApplied) restore() error {
	var result error
	for path, data := range a.before {
		result = errors.Join(result, a.store.writeFile(path, data))
	}
	if a.created != "" {
		root, err := os.OpenRoot(a.store.managedDir)
		if err != nil {
			return errors.Join(result, err)
		}
		defer root.Close()
		result = errors.Join(result, root.Remove(filepath.Base(a.created)))
	}
	return result
}

func (a *PolicyApplied) Finish(ctx context.Context, success bool) error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	p, err := a.store.readPolicy("")
	if err != nil {
		return err
	}
	if p.hash != a.hash || p.document.hash != a.configHash {
		return protocol.NewError(protocol.CodeConflict, "configuration changed during policy application; refusing to overwrite it")
	}
	if success {
		_, err := a.store.db.ExecContext(ctx, "DELETE FROM policy_draft WHERE id=1")
		return err
	}
	if err := a.restore(); err != nil {
		return err
	}
	d, err := readDocument(a.store.path)
	if err != nil {
		return err
	}
	if err := a.store.advanceDrafts(ctx, a.configHash, d.hash); err != nil {
		return err
	}
	_, err = a.store.db.ExecContext(ctx, "UPDATE policy_draft SET source=? WHERE id=1", a.source)
	return err
}
