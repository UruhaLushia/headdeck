package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type Store struct {
	db         *sql.DB
	path       string
	managedDir string
	mu         sync.Mutex
}

func New(db *sql.DB, path string) (*Store, error) {
	return NewInDirectory(db, path, ManagedDir)
}

func NewInDirectory(db *sql.DB, path, managedDir string) (*Store, error) {
	if path == "" {
		path = DefaultPath
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS dns_draft (id INTEGER PRIMARY KEY CHECK(id=1), source TEXT NOT NULL, value TEXT NOT NULL, restart INTEGER NOT NULL)`); err != nil {
		return nil, err
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS managed_draft (id INTEGER PRIMARY KEY CHECK(id=1), source TEXT NOT NULL, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS policy_draft (id INTEGER PRIMARY KEY CHECK(id=1), source TEXT NOT NULL, value TEXT NOT NULL)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return nil, err
		}
	}
	return &Store{db: db, path: path, managedDir: managedDir}, nil
}

func (s *Store) get(ctx context.Context) (*protocol.DNSSettings, *document, error) {
	d, err := readDocument(s.path)
	if err != nil {
		return nil, nil, protocol.NewError(protocol.CodeClientConfig, "%s", err)
	}
	out := &protocol.DNSSettings{Config: d.config, Writable: writable(s.path, s.managedDir) && (d.external == "" || writable(d.external, s.managedDir)), ExternalRecords: d.external != ""}
	var source, value string
	var restart bool
	err = s.db.QueryRowContext(ctx, "SELECT source,value,restart FROM dns_draft WHERE id=1").Scan(&source, &value, &restart)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	if err == nil {
		if source != d.hash {
			return nil, nil, protocol.NewError(protocol.CodeConflict, "Headscale configuration changed outside Headdeck; discard the DNS draft and refresh")
		}
		if err := json.Unmarshal([]byte(value), &out.Config); err != nil {
			return nil, nil, err
		}
		out.Pending = true
	}
	out.Revision = fingerprint([]byte(d.hash), []byte(value))
	out.RestartRequired = out.Pending && restart
	return out, d, nil
}

func (s *Store) Get(ctx context.Context) (*protocol.DNSSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, _, err := s.get(ctx)
	return out, err
}

func (s *Store) Save(ctx context.Context, req protocol.DNSUpdate) (*protocol.DNSSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, d, err := s.get(ctx)
	if err != nil {
		return nil, err
	}
	if req.Revision != out.Revision {
		return nil, protocol.NewError(protocol.CodeConflict, "DNS settings changed; refresh before saving")
	}
	if !out.Writable {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "DNS configuration is read-only; authorize configuration management through sudo")
	}
	if err := Validate(req.Config, d.serverURL); err != nil {
		return nil, protocol.NewError(protocol.CodeBadRequest, "%s", err)
	}
	normalizeDNS(&req.Config)
	value, err := json.Marshal(req.Config)
	if err != nil {
		return nil, err
	}
	restart := out.RestartRequired || d.external == "" || restartRequired(d.config, req.Config)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO dns_draft(id,source,value,restart) VALUES(1,?,?,?) ON CONFLICT(id) DO UPDATE SET source=excluded.source,value=excluded.value,restart=excluded.restart`, d.hash, string(value), restart); err != nil {
		return nil, err
	}
	out, _, err = s.get(ctx)
	return out, err
}

func (s *Store) Discard(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, "DELETE FROM dns_draft WHERE id=1")
	return err
}

type Applied struct {
	RestartRequired bool
	store           *Store
	before          map[string][]byte
	hash            string
	table           string
}

func (s *Store) Apply(ctx context.Context, revision string) (*Applied, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, d, err := s.get(ctx)
	if err != nil {
		return nil, err
	}
	if out.Revision != revision {
		return nil, protocol.NewError(protocol.CodeConflict, "DNS settings changed; refresh before applying")
	}
	if !out.Pending {
		return nil, protocol.NewError(protocol.CodeBadRequest, "no pending DNS changes")
	}
	if !out.Writable {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "DNS configuration is read-only")
	}
	if err := Validate(out.Config, d.serverURL); err != nil {
		return nil, err
	}
	data, err := d.render(out.Config)
	if err != nil {
		return nil, err
	}
	a := &Applied{store: s, before: make(map[string][]byte), RestartRequired: out.RestartRequired}
	if d.external != "" {
		a.before[d.external] = d.files[d.external]
		records, err := json.MarshalIndent(out.Config.ExtraRecords, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := s.writeFile(d.external, append(records, '\n')); err != nil {
			return nil, err
		}
	}
	if out.RestartRequired {
		a.before[s.path] = d.files[s.path]
		if err := s.writeFile(s.path, data); err != nil {
			return nil, errors.Join(err, a.restore())
		}
	}
	updated, err := readDocument(s.path)
	if err != nil {
		return nil, errors.Join(err, a.restore())
	}
	a.hash = updated.hash
	// Keep the pending state recoverable across daemon restarts until API health succeeds.
	if err := s.advanceDrafts(ctx, d.hash, a.hash); err != nil {
		return nil, errors.Join(err, a.restore())
	}
	return a, nil
}

func (a *Applied) restore() error {
	var result error
	for path, data := range a.before {
		result = errors.Join(result, a.store.writeFile(path, data))
	}
	return result
}

func (a *Applied) Finish(ctx context.Context, success bool) error {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	d, err := readDocument(a.store.path)
	if err != nil {
		return err
	}
	if d.hash != a.hash {
		return fmt.Errorf("headscale configuration changed during DNS application; refusing to overwrite it")
	}
	if success {
		_, err := a.store.db.ExecContext(ctx, "DELETE FROM "+a.draftTable()+" WHERE id=1")
		return err
	}
	if err := a.restore(); err != nil {
		return err
	}
	d, err = readDocument(a.store.path)
	if err != nil {
		return err
	}
	return a.store.advanceDrafts(ctx, a.hash, d.hash)
}

func (a *Applied) draftTable() string {
	if a.table == "managed_draft" {
		return a.table
	}
	return "dns_draft"
}

func restartRequired(before, after protocol.DNSConfig) bool {
	before.ExtraRecords, after.ExtraRecords = nil, nil
	return !reflect.DeepEqual(before, after)
}
