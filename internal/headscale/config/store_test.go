package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"go.yaml.in/yaml/v3"
)

func TestManagedWritesCannotEscape(t *testing.T) {
	dir, outside := t.TempDir(), filepath.Join(t.TempDir(), "target")
	original := []byte("untouched")
	if err := os.WriteFile(outside, original, 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Store{managedDir: dir}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, link, filepath.Join(dir, "..", filepath.Base(outside))} {
		if err := s.writeFile(path, []byte("overwrite")); err == nil {
			t.Fatal("write escaped the managed directory", path)
		}
	}
	data, err := os.ReadFile(outside)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("outside file was modified", err)
	}
	executable := filepath.Join(dir, "program")
	if err := os.WriteFile(executable, original, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.writeFile(executable, []byte("overwrite")); err == nil {
		t.Fatal("configuration write replaced executable content")
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Symlink(configPath, alias); err != nil {
		t.Fatal(err)
	}
	if err := s.writeFile(alias, []byte("managed")); err != nil {
		t.Fatal("authorized configuration symlink rejected", err)
	}
}

func TestDNSDraftApplyRollbackAndConflict(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline", true: "external"}[external], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			original := []byte("# preserved\nserver_url: https://control.example.test\noidc:\n  client_secret: never-expose-this\nunknown_option: retained\ndns:\n  magic_dns: true\n  base_domain: vpn\n  override_local_dns: true\n  nameservers:\n    global: [1.1.1.1]\n    split: {}\n  unknown_dns: retained\n  search_domains: []\n  extra_records: []\n")
			if external {
				records := filepath.Join(dir, "records.json")
				if err := os.WriteFile(records, []byte("[]"), 0o600); err != nil {
					t.Fatal(err)
				}
				original = append(original, []byte("  extra_records_path: "+records+"\n")...)
			}
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			db, err := config.Open(filepath.Join(dir, "settings.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			s, err := New(db.Database(), path)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			state, err := s.Get(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if state.Writable {
				t.Fatal("unmanaged configuration was writable")
			}
			s.managedDir = dir
			state, err = s.Get(ctx)
			if err != nil {
				t.Fatal(err)
			}
			req := protocol.DNSUpdate{Config: state.Config, Revision: state.Revision}
			req.Config.ExtraRecords = []protocol.DNSRecord{{Name: "app.example.test", Type: "AAAA", Value: "fd00::1"}}
			state, err = s.Save(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if !state.Pending || state.RestartRequired == external {
				t.Fatalf("wrong pending/restart state: %+v", state)
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, original) {
				t.Fatal("saving a draft wrote the live configuration")
			}
			if _, err := s.Save(ctx, req); err == nil {
				t.Fatal("stale save accepted")
			}
			reopened, err := New(db.Database(), path)
			if err != nil {
				t.Fatal(err)
			}
			reopened.managedDir = dir
			state, err = reopened.Get(ctx)
			if err != nil || !state.Pending {
				t.Fatal("draft did not persist")
			}
			applied, err := reopened.Apply(ctx, state.Revision)
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if external && !bytes.Equal(data, original) {
				t.Fatal("record-only application rewrote the main configuration")
			}
			var doc map[string]any
			if err := yaml.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			if doc["unknown_option"] != "retained" || doc["oidc"].(map[string]any)["client_secret"] != "never-expose-this" || doc["dns"].(map[string]any)["unknown_dns"] != "retained" || !bytes.Contains(data, []byte("# preserved")) {
				t.Fatal("unrelated fields or comments were lost")
			}
			if err := applied.Finish(ctx, false); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(data, original) {
				t.Fatal("rollback did not restore exact bytes")
			}
			state, err = reopened.Get(ctx)
			if err != nil {
				t.Fatal(err)
			}
			applied, err = reopened.Apply(ctx, state.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if err := applied.Finish(ctx, true); err != nil {
				t.Fatal(err)
			}
			state, err = reopened.Get(ctx)
			if err != nil || state.Pending || len(state.Config.ExtraRecords) != 1 {
				t.Fatal("successful apply did not clear the draft")
			}
			req = protocol.DNSUpdate{Config: state.Config, Revision: state.Revision}
			req.Config.BaseDomain = "new.example.test"
			state, err = reopened.Save(ctx, req)
			if err != nil || !state.RestartRequired {
				t.Fatalf("base domain change must restart: %v", err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(data, []byte("# external edit\n")...), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Apply(ctx, state.Revision); err == nil {
				t.Fatal("external configuration edit was overwritten")
			}
			if err := reopened.Discard(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Get(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
