package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"

	"github.com/UruhaLushia/headdeck/internal/securefile"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("database path must be absolute")
	}
	if err := securefile.PrepareDirectory(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err == nil {
		f.Close()
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || !securefile.Owned(info) {
		return nil, fmt.Errorf("settings database must be a regular file with mode 0600")
	}
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initialize() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return fmt.Errorf("unsupported settings schema %d", version)
	}
	_, err := s.db.Exec("CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL); PRAGMA user_version=1;")
	if err != nil {
		return err
	}
	return s.ensureOIDCSecret()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Database() *sql.DB { return s.db }

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func read(ctx context.Context, db queryer) (*Config, error) {
	rows, err := db.QueryContext(ctx, "SELECT key, value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c := Default()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if err := set(c, key, value); err != nil {
			return nil, err
		}
	}
	c.manageOIDC()
	return c, rows.Err()
}

func (s *Store) Load(ctx context.Context) (*Config, error) { return read(ctx, s.db) }

func Load(path string) (*Config, error) {
	s, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return s.Load(context.Background())
}

func (s *Store) Update(ctx context.Context, changes map[string]string) (*Config, error) {
	return s.UpdateValidated(ctx, changes, nil)
}
func (s *Store) UpdateValidated(ctx context.Context, changes map[string]string, validate func(*Config) error) (*Config, error) {
	for key := range changes {
		if ProtectedSetting(key) {
			return nil, fmt.Errorf("setting %q requires local installation configuration", key)
		}
	}
	return s.update(ctx, changes, validate)
}

func (s *Store) update(ctx context.Context, changes map[string]string, validate func(*Config) error) (*Config, error) {
	changes = maps.Clone(changes)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	c, err := read(ctx, tx)
	if err != nil {
		return nil, err
	}
	secret := c.Server.Web.OIDCServer.ClientSecret
	for key, value := range changes {
		if err := set(c, key, value); err != nil {
			return nil, err
		}
	}
	if c.Server.Web.OIDCServer.ClientSecret == "" {
		c.Server.Web.OIDCServer.ClientSecret = secret
	}
	c.manageOIDC()
	current := values(c)
	for key := range changes {
		changes[key] = current[key]
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if validate != nil {
		if err := validate(c); err != nil {
			return nil, err
		}
	}
	for key, value := range changes {
		if _, err := tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

func Save(path string, c *Config) error {
	s, err := Open(path)
	if err != nil {
		return err
	}
	defer s.Close()
	_, err = s.update(context.Background(), values(c), nil)
	return err
}
