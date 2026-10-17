package daemon_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/daemon"
	"github.com/UruhaLushia/headdeck/internal/ipc/client"
)

type fixture struct {
	client   *client.Client
	grants   *grants.Store
	database string
}

func start(t *testing.T, mock ...http.HandlerFunc) *fixture {
	t.Helper()
	base, err := filepath.Abs("../../.dev/testtmp")
	if err != nil {
		t.Fatal(err)
	}
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		base = filepath.Join(runtime, "headdeck-tests")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "uid-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if len(mock) > 0 {
			mock[0](w, r)
			return
		}
		body := map[string]any{}
		switch r.URL.Path {
		case "/version":
			body["version"] = "0.29.4"
		case "/api/v1/node":
			body["nodes"] = []any{map[string]any{"id": "1", "givenName": "node-one"}}
		case "/api/v1/user":
			body["users"] = []any{}
		case "/api/v1/apikey":
			body["apiKeys"] = []any{}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(hs.Close)
	cfg := config.Default()
	cfg.Headscale.BaseURL = hs.URL
	cfg.Headscale.APIKey = "test-key"
	cfg.Features.ServiceControlDriver = "none"
	database := filepath.Join(dir, "settings.db")
	if err := config.Save(database, cfg); err != nil {
		t.Fatal(err)
	}
	store := &grants.Store{Path: filepath.Join(dir, "users.json"), OwnerUID: uint32(os.Getuid())}
	socket := filepath.Join(dir, "ipc.sock")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		for {
			loaded, err := config.Load(database)
			if err != nil {
				done <- err
				return
			}
			loaded.IPC.Listen = socket
			loaded.IPC.SocketMode = "0600"
			d, err := daemon.New(daemon.Options{Config: loaded, ConfigPath: database, Authorization: store, LogLevel: "error"})
			if err != nil {
				done <- err
				return
			}
			err = d.Run(ctx)
			if !errors.Is(err, daemon.ErrReload) {
				done <- err
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	c, err := client.New(client.Options{SocketPath: socket, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := c.Ping(ctx); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return &fixture{client: c, grants: store, database: database}
}

func (f *fixture) authorize(t *testing.T, role principal.Role) {
	t.Helper()
	err := f.grants.Update(os.Getgid(), func([]grants.User) ([]grants.User, error) {
		if role == "" {
			return []grants.User{}, nil
		}
		return []grants.User{{UID: uint32(os.Getuid()), Name: "test", Role: role}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUIDAuthorizationAndRevocation(t *testing.T) {
	f := start(t)
	ctx := context.Background()
	if _, err := f.client.Status(ctx, false); err == nil {
		t.Fatal("daemon uid must not have implicit access")
	}
	f.authorize(t, principal.RoleAdmin)
	if nodes, err := f.client.Nodes(ctx, ""); err != nil || len(nodes.Nodes) != 1 {
		t.Fatalf("authorized nodes: %v %v", nodes, err)
	}
	started := make(chan struct{}, 1)
	ended := make(chan error, 1)
	go func() {
		ended <- f.client.WatchEvents(ctx, client.WatchOptions{OnCursor: func(uint64) {
			select {
			case started <- struct{}{}:
			default:
			}
		}})
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("SSE did not open")
	}
	f.authorize(t, principal.RoleMember)
	if _, err := f.client.UpdateSettings(ctx, map[string]string{"headscale.timeout": "2s"}); err == nil {
		t.Fatal("member changed settings")
	}
	select {
	case err := <-ended:
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("revoked event stream: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("role change did not close SSE")
	}
	f.authorize(t, "")
	if _, err := f.client.Nodes(ctx, ""); err == nil {
		t.Fatal("revoked user used a cached handshake")
	}
}

func TestSQLiteSettingsAndReload(t *testing.T) {
	f := start(t)
	f.authorize(t, principal.RoleOwner)
	ctx := context.Background()
	before, err := f.client.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range before.Items {
		if s.Secret && s.Value != "" {
			t.Fatal("secret leaked")
		}
	}
	if _, err := f.client.UpdateSettings(ctx, map[string]string{"auth.users": "/evil", "headscale.timeout": "3s"}); err == nil {
		t.Fatal("authorization can be changed through settings")
	}
	loaded, err := config.Load(f.database)
	if err != nil || loaded.Headscale.Timeout != 8*time.Second {
		t.Fatal("failed update was not atomic", err)
	}
	if _, err := f.client.UpdateSettings(ctx, map[string]string{"headscale.base_url": "file:///etc/passwd"}); err == nil {
		t.Fatal("invalid URL accepted")
	}
	changed, err := f.client.UpdateSettings(ctx, map[string]string{"server.log_level": "warn", "server.web.github.client_secret": "replacement-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed.ReloadRequired {
		t.Fatal("changed settings not pending")
	}
	blob, _ := json.Marshal(changed)
	if strings.Contains(string(blob), "replacement-secret") {
		t.Fatal("settings response leaked the secret")
	}
	for _, s := range changed.Items {
		if s.Secret && (s.Value != "" || (s.Key == "server.web.github.client_secret" && !s.Configured)) {
			t.Fatal("secret exposed or missing")
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		current, err := f.client.Settings(ctx)
		if err == nil && !current.ReloadRequired {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("reload failed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	loaded, err = config.Load(f.database)
	if err != nil || loaded.Server.LogLevel != "warn" || loaded.Server.Web.GitHub.ClientSecret != "replacement-secret" {
		t.Fatal("settings did not persist", err)
	}
}

func TestLocalRequestChecks(t *testing.T) {
	f := start(t)
	f.authorize(t, principal.RolePending)
	for _, check := range []struct{ host, header, value string }{
		{"evil.example", "", ""}, {"localhost", "", ""}, {"headdeck.local:80", "", ""},
		{"headdeck.local", "Origin", "https://evil.example"}, {"headdeck.local", "Origin", "null"},
		{"headdeck.local", "Referer", "http://headdeck.local/"}, {"headdeck.local", "Origin", ""},
	} {
		for _, path := range []string{"/ping", "/api/v1/users"} {
			req, err := http.NewRequest(http.MethodGet, "http://headdeck.local"+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = check.host
			if check.header != "" {
				req.Header.Set(check.header, check.value)
			}
			resp, err := f.client.HTTPClient().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s %s: got %d", path, check.host, check.header, resp.StatusCode)
			}
		}
	}
	f.authorize(t, principal.RoleAdmin)
	if _, err := f.client.Users(context.Background()); err != nil {
		t.Fatal("valid local request rejected", err)
	}
}
