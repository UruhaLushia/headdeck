package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"go.yaml.in/yaml/v3"
)

func TestDiagnosticsListener(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	for _, test := range []struct{ value, want string }{
		{"{}", "http://127.0.0.1:9090"},
		{"metrics_listen_addr: 0.0.0.0:6762", "http://127.0.0.1:6762"},
		{"metrics_listen_addr: '[::]:6762'", "http://[::1]:6762"},
		{"metrics_listen_addr: localhost:6762", "http://127.0.0.1:6762"},
		{"metrics_listen_addr: ''", ""},
		{"metrics_listen_addr: example.com:9090", ""},
		{"metrics_listen_addr: 192.0.2.1:9090", ""},
		{"metrics_listen_addr: '127.0.0.1:0'", ""},
	} {
		if err := os.WriteFile(path, []byte(test.value), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := DiagnosticsUpstream(path)
		if got != test.want || (err != nil) != (test.want == "") {
			t.Fatalf("%s: got %q, %v", test.value, got, err)
		}
	}
}

func TestManagedPrefixes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("server_url: https://control.example\ndns: {}\nprefixes:\n  v4: 100.64.0.0/10 # retained\n  allocation: sequential\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	db, err := config.Open(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewInDirectory(db.Database(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	managed, err := s.Managed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range managed.Items {
		seen[item.Key] = true
	}
	if !seen["hs.prefixes.v4"] || !seen["hs.prefixes.v6"] || !seen["hs.prefixes.allocation"] {
		t.Fatal("prefix settings missing, including the unconfigured family")
	}
	for _, values := range []map[string]string{
		{"hs.prefixes.v4": "192.0.2.1"}, {"hs.prefixes.v4": "fd7a:115c:a1e0::/48"},
		{"hs.prefixes.v6": "100.64.0.0/10"}, {"hs.prefixes.v6": "::ffff:100.64.0.0/106"},
		{"hs.prefixes.v4": ""}, {"hs.prefixes.v4": "", "hs.prefixes.v6": ""},
	} {
		if _, err := s.SaveManaged(ctx, protocol.ManagedConfigUpdate{Revision: managed.Revision, Values: values}); err == nil {
			t.Fatal("invalid prefixes accepted", values)
		}
	}
	managed, err = s.SaveManaged(ctx, protocol.ManagedConfigUpdate{Revision: managed.Revision, Values: map[string]string{
		"hs.prefixes.v4": "100.64.0.0/24", "hs.prefixes.v6": "fd7a:115c:a1e0::/64", "hs.prefixes.allocation": "random",
	}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, original) {
		t.Fatal("saving a draft changed the live configuration", err)
	}
	applied, err := s.ApplyManaged(ctx, managed.Revision)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Prefixes struct{ V4, V6, Allocation string }
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil || parsed.Prefixes.V4 != "100.64.0.0/24" || parsed.Prefixes.V6 != "fd7a:115c:a1e0::/64" || parsed.Prefixes.Allocation != "random" || !bytes.Contains(data, []byte("# retained")) {
		t.Fatal("incorrect applied prefix configuration", err)
	}
	if err := applied.Finish(ctx, false); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("prefix rollback did not restore the original", err)
	}
}

func TestOIDCTakeoverValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	db, err := config.Open(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewInDirectory(db.Database(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	w := config.Default().Server.Web
	w.PublicURL = "https://control.example"
	w.OIDCServer.ClientSecret = strings.Repeat("s", 32)
	w.OIDCServer.RedirectURI = "https://control.example/oidc/callback"
	valid := "server_url: https://control.example\nlisten_addr: 127.0.0.1:8080\ndns: {}\noidc:\n  issuer: https://control.example/oidc\n  client_id: headscale\n  client_secret: " + w.OIDCServer.ClientSecret + "\n  scope: [openid, profile, email]\n  pkce:\n    enabled: true\n    method: S256\n"
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"matching", valid, true},
		{"broken DNS can still be repaired", strings.ReplaceAll(valid, "dns: {}", "dns: {extra_records_path: /missing-records.json}"), true},
		{"old issuer", strings.ReplaceAll(valid, "https://control.example/oidc", "https://dex.example"), false},
		{"different public entry", strings.ReplaceAll(valid, "server_url: https://control.example", "server_url: https://other.example"), false},
		{"public listener", strings.ReplaceAll(valid, "127.0.0.1:8080", "0.0.0.0:8080"), false},
		{"upstream TLS", valid + "tls_cert_path: /etc/headscale/cert.pem\n", false},
		{"wrong secret", strings.ReplaceAll(valid, w.OIDCServer.ClientSecret, "other-secret"), false},
		{"pkce disabled", strings.ReplaceAll(valid, "enabled: true", "enabled: false"), false},
		{"upstream authorization", strings.ReplaceAll(valid, "  scope:", "  allowed_users: [owner@example.com]\n  scope:"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.text), 0o640); err != nil {
				t.Fatal(err)
			}
			if err := s.CheckOIDC(w); (err == nil) != tc.want {
				t.Fatalf("validation: %v", err)
			}
		})
	}
}

func TestOIDCTakeoverApplyAndRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("# preserve\nserver_url: https://old.example\nlisten_addr: 0.0.0.0:6761 # internal\ndns: {}\ndatabase:\n  type: sqlite\n  sqlite:\n    path: /var/lib/headscale/db.sqlite\noidc:\n  issuer: https://dex.example\n  expiry: 180d\n  allowed_users: [old@example.com]\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	db, err := config.Open(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewInDirectory(db.Database(), path, dir)
	if err != nil {
		t.Fatal(err)
	}
	w := config.Default().Server.Web
	w.PublicURL = "https://control.example"
	w.OIDCServer.RedirectURI = "https://control.example/oidc/callback"
	w.OIDCServer.ClientSecret = strings.Repeat("s", 32)
	change, err := s.PlanOIDC(w)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.WriteOIDC(ctx, *change, false); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckOIDC(w); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(change.After, []byte("server_url: https://control.example")) || !bytes.Contains(change.After, []byte("listen_addr: 127.0.0.1:6761 # internal")) {
		t.Fatal("public URL and internal listener were not managed")
	}
	next, err := s.PlanOIDC(w)
	if err != nil || !bytes.Equal(next.Before, next.After) {
		t.Fatal("repeated takeover would restart Headscale", err)
	}
	managed, err := s.Managed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hs.server_url", "hs.listen_addr"} {
		for _, item := range managed.Items {
			if item.Key == key {
				t.Fatal("managed connection exposed as an editable setting", key)
			}
		}
		if _, err := s.SaveManaged(ctx, protocol.ManagedConfigUpdate{Revision: managed.Revision, Values: map[string]string{key: "https://other.example"}}); err == nil {
			t.Fatal("managed connection could be overridden", key)
		}
	}
	if !bytes.Contains(change.After, []byte("expiry: 180d")) || !bytes.Contains(change.After, []byte("# preserve")) || bytes.Contains(change.After, []byte("allowed_users")) {
		t.Fatal("wrong OIDC configuration")
	}
	if err := s.WriteOIDC(ctx, *change, true); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOIDC(ctx, *change, true); err != nil {
		t.Fatal("rollback must be idempotent", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("original configuration was not restored")
	}
	if err := os.WriteFile(path, append(original, []byte("# external change\n")...), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOIDC(ctx, *change, false); err == nil {
		t.Fatal("external changes were overwritten")
	}
}

func TestHeadscaleUpstream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	for _, tc := range []struct{ name, text, want string }{
		{"default", "dns: {}", "http://127.0.0.1:8080"},
		{"loopback", "listen_addr: 127.0.0.1:6761", "http://127.0.0.1:6761"},
		{"wildcard", "listen_addr: 0.0.0.0:6761", "http://127.0.0.1:6761"},
		{"IPv6", "listen_addr: '[::]:6761'", "http://[::1]:6761"},
		{"public interface", "listen_addr: 192.0.2.1:6761", ""},
		{"invalid port", "listen_addr: 127.0.0.1:99999", ""},
		{"listener collision", "listen_addr: 0.0.0.0:8088", ""},
		{"TLS", "listen_addr: 127.0.0.1:6761\ntls_key_path: /etc/headscale/key.pem", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.text), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := Upstream(path, "127.0.0.1:8088")
			if (err == nil) != (tc.want != "") || got != tc.want {
				t.Fatalf("upstream %q, error %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestIndependentDraftsAndPolicyRollback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("# preserved\nserver_url: https://control.example.test\nlog:\n  level: info\ndns:\n  magic_dns: true\n  base_domain: vpn.example.test\n  override_local_dns: true\n  nameservers:\n    global: [1.1.1.1]\n    split: {}\n  search_domains: []\n  extra_records: []\npolicy:\n  mode: file\n  path: ''\n")
	external := filepath.Join(dir, "records.json")
	if err := os.WriteFile(external, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(external)
	if err != nil {
		t.Fatal(err)
	}
	original = bytes.Replace(original, []byte("policy:\n"), []byte("  extra_records_path: "+external+"\npolicy:\n"), 1)
	if err := os.WriteFile(path, original, 0o640); err != nil {
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
	s.managedDir = dir
	ctx := context.Background()
	dns, err := s.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dns.Config.SearchDomains = []string{"example.test"}
	if _, err = s.Save(ctx, protocol.DNSUpdate{Config: dns.Config, Revision: dns.Revision}); err != nil {
		t.Fatal(err)
	}
	managed, err := s.Managed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	managed, err = s.SaveManaged(ctx, protocol.ManagedConfigUpdate{Values: map[string]string{"hs.log.level": "debug"}, Revision: managed.Revision})
	if err != nil {
		t.Fatal(err)
	}
	apply, err := s.ApplyManaged(ctx, managed.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx); err != nil {
		t.Fatal("applying config invalidated the DNS draft", err)
	}
	if err = apply.Finish(ctx, false); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(external)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("managed rollback rewrote unrelated external records", err)
	}
	policy, err := s.Policy(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = s.SavePolicy(ctx, protocol.PolicyUpdate{Policy: `{"acls":[]}`, Revision: policy.Revision}, "")
	if err != nil {
		t.Fatal(err)
	}
	applied, err := s.ApplyPolicyFile(ctx, policy.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Managed(ctx); err != nil {
		t.Fatal("policy creation invalidated the config draft", err)
	}
	if _, err = s.Get(ctx); err != nil {
		t.Fatal("policy creation invalidated the DNS draft", err)
	}
	if err = applied.Finish(ctx, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("rollback did not restore the original config", err)
	}
	if _, err = os.Stat(filepath.Join(dir, "policy.hujson")); !os.IsNotExist(err) {
		t.Fatal("rollback leaked the new policy file", err)
	}
	if _, err = s.Policy(ctx, ""); err != nil {
		t.Fatal("rollback invalidated the policy draft", err)
	}
	if _, err = s.Managed(ctx); err != nil {
		t.Fatal("rollback invalidated the config draft", err)
	}
	if _, err = s.Get(ctx); err != nil {
		t.Fatal("rollback invalidated the DNS draft", err)
	}
}
