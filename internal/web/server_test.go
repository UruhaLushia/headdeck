package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/events"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"github.com/UruhaLushia/headdeck/internal/service"
)

func TestReadOnlyDiagnosticsAndAuthorization(t *testing.T) {
	var reads atomic.Int64
	var processStart atomic.Int64
	processStart.Store(1000)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Forwarded-For") != "" {
			t.Error("credentials forwarded to diagnostics")
		}
		switch r.URL.Path {
		case "/metrics":
			_, _ = w.Write([]byte("go_goroutines 42\n" +
				"http_requests_inflight{host=\"space in label\",proto=\"HTTP/1.1\"} 3\n" +
				"http_requests_inflight{proto=\"HTTP/2.0\"} 4\n" +
				"process_start_time_seconds " + strconv.FormatInt(processStart.Load(), 10) + "\n" +
				"process_open_fds NaN\ngo_memstats_heap_alloc_bytes +Inf\nunknown_metric 999\n"))
		case "/debug/overview":
			_, _ = w.Write([]byte(`{"nodes":{"total":22}}`))
		case "/debug/pprof/goroutine":
			if r.URL.RawQuery != "debug=1" {
				t.Error("unbounded profile requested")
			}
			_, _ = w.Write([]byte("goroutine profile: total 42"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	path := filepath.Join(t.TempDir(), "headscale.yaml")
	if err := os.WriteFile(path, []byte("metrics_listen_addr: "+strings.TrimPrefix(upstream.URL, "http://")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, _, _ := testWebServer(t, upstream.URL, path)
	admin := login(t, srv)
	member := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, "member", principal.RoleMember, srv.now())}
	operator := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, "administrator", principal.RoleAdmin, srv.now())}
	for _, cookie := range []*http.Cookie{nil, member} {
		var cookies []*http.Cookie
		want := http.StatusUnauthorized
		if cookie != nil {
			cookies = []*http.Cookie{cookie}
			want = http.StatusForbidden
		}
		out := request(t, srv, "GET", "/api/headscale/diagnostics/metrics", "", nil, cookies)
		if out.Code != want || reads.Load() != 0 {
			t.Fatal("diagnostics authorization bypass", out.Code, reads.Load())
		}
	}
	for _, section := range []string{"metrics", "overview", "goroutine"} {
		out := request(t, srv, "GET", "/api/headscale/diagnostics/"+section, "", nil, []*http.Cookie{admin})
		if out.Code != http.StatusForbidden || reads.Load() != 0 {
			t.Fatal("diagnostics must be disabled by default", section, out.Code)
		}
	}
	for _, cookie := range []*http.Cookie{member, operator} {
		out := request(t, srv, "PATCH", "/api/settings", `{"changes":{"server.diagnostics":"true"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
		want := http.StatusForbidden
		if cookie == operator {
			want = http.StatusOK
		}
		if out.Code != want || reads.Load() != 0 {
			t.Fatal("incorrect diagnostic opt-in authorization", out.Code, out.Body.String())
		}
	}
	if out := request(t, srv, "GET", "/api/headscale/diagnostics/metrics", "", nil, []*http.Cookie{member}); out.Code != http.StatusForbidden || reads.Load() != 0 {
		t.Fatal("member accessed enabled diagnostics", out.Code)
	}
	for _, section := range []string{"config", "gc", "ping", "profile", "trace", "registration-cache"} {
		out := request(t, srv, "GET", "/api/headscale/diagnostics/"+section, "", nil, []*http.Cookie{admin})
		if out.Code != http.StatusBadRequest || reads.Load() != 0 {
			t.Fatal("unsafe debug endpoint reached", section, out.Code, reads.Load())
		}
	}
	for _, section := range []string{"metrics", "metrics", "overview", "goroutine"} {
		out := request(t, srv, "GET", "/api/headscale/diagnostics/"+section+"?seconds=100&url=http://example.com", "", nil, []*http.Cookie{admin})
		var data protocol.Diagnostic
		if out.Code != http.StatusOK || json.Unmarshal(out.Body.Bytes(), &data) != nil || data.CollectedAt.IsZero() || data.Section != section || out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid diagnostic response", out.Code, out.Body.String())
		}
		if (data.Format == "json") != (section == "overview") {
			t.Fatal("incorrect diagnostic format", data.Format)
		}
		if section == "metrics" {
			if data.Content != "" || strings.Contains(out.Body.String(), `"content"`) {
				t.Fatal("raw metrics included in chart response")
			}
			if len(data.Samples) != 1 || !data.Samples[0].Time.Equal(data.CollectedAt) {
				t.Fatal("cached metrics added fabricated samples", data.Samples)
			}
			values := data.Samples[0].Values
			if len(values) != 3 || values["go_goroutines"] != 42 || values["http_requests_inflight"] != 7 || values["process_start_time_seconds"] != 1000 {
				t.Fatal("incorrect metric aggregation", values)
			}
		} else if len(data.Samples) != 0 || data.Content == "" {
			t.Fatal("invalid debug output")
		}
	}
	if reads.Load() != 3 {
		t.Fatal("diagnostics did not share snapshots", reads.Load())
	}
	for _, step := range []struct {
		prune   bool
		restart bool
		want    int
	}{
		{want: 2},
		{prune: true, want: 2},
		{restart: true, want: 1},
	} {
		cached, err := srv.svc.Diagnostic(context.Background(), "metrics")
		if err != nil {
			t.Fatal(err)
		}
		cached.CollectedAt = time.Now().Add(-11 * time.Second)
		if step.prune {
			cached.Samples[0].Time = time.Now().Add(-2 * time.Hour)
		}
		if step.restart {
			processStart.Store(2000)
		}
		out := request(t, srv, "GET", "/api/headscale/diagnostics/metrics", "", nil, []*http.Cookie{admin})
		var data protocol.Diagnostic
		if out.Code != http.StatusOK || json.Unmarshal(out.Body.Bytes(), &data) != nil || len(data.Samples) != step.want {
			t.Fatal("invalid metric history", step, out.Body.String())
		}
	}
	if reads.Load() != 6 {
		t.Fatal("metric history did not use actual upstream reads", reads.Load())
	}
	out := request(t, srv, "PATCH", "/api/settings", `{"changes":{"server.diagnostics":"false"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{admin})
	if out.Code != http.StatusOK {
		t.Fatal("cannot disable diagnostics", out.Body.String())
	}
	for _, section := range []string{"metrics", "overview", "goroutine"} {
		out := request(t, srv, "GET", "/api/headscale/diagnostics/"+section, "", nil, []*http.Cookie{admin})
		if out.Code != http.StatusForbidden || reads.Load() != 6 {
			t.Fatal("disabled diagnostics served cached data or read upstream", section, out.Code)
		}
	}
}

const testToken = "correct-horse-battery-staple"

func TestServiceLogsAuthorizationAndBounds(t *testing.T) {
	dir := t.TempDir()
	arguments := filepath.Join(dir, "arguments")
	data := filepath.Join(dir, "journal.json")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$JOURNAL_ARGS\"\nexec /usr/bin/cat \"$JOURNAL_DATA\"\n"
	if err := os.WriteFile(filepath.Join(dir, "journalctl"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("JOURNAL_ARGS", arguments)
	t.Setenv("JOURNAL_DATA", data)
	fixture := `{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"6","MESSAGE":"{\"level\":\"error\",\"msg\":\"hskey-secret\"}"}
{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"6","MESSAGE":"time=now level=WARN msg=retry"}
{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"7","MESSAGE":"\u001b[31mdebug message\u001b[0m"}
{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"6","MESSAGE":[98,105,110,97,114,121]}
{"__REALTIME_TIMESTAMP":"1700000000000000","PRIORITY":"6","MESSAGE":"2026-10-18T10:05:00Z \u001b[31mERR\u001b[0m failed to start"}
`
	if err := os.WriteFile(data, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, svc, _ := testWebServer(t)
	for _, role := range []principal.Role{"", principal.RolePending, principal.RoleMember} {
		var cookies []*http.Cookie
		want := http.StatusUnauthorized
		if role != "" {
			want = http.StatusForbidden
			cookies = []*http.Cookie{{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, string(role), role, srv.now())}}
		}
		if out := request(t, srv, "GET", "/api/logs/headdeck", "", nil, cookies); out.Code != want {
			t.Fatalf("%s log access: %d", role, out.Code)
		}
	}
	owner := login(t, srv)
	if out := request(t, srv, "GET", "/api/logs/sshd.service", "", nil, []*http.Cookie{owner}); out.Code != http.StatusBadRequest {
		t.Fatal("arbitrary log source accepted", out.Code)
	}
	if _, err := os.Stat(arguments); !os.IsNotExist(err) {
		t.Fatal("rejected request executed journalctl")
	}
	admin := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, "administrator", principal.RoleAdmin, srv.now())}
	svc.Config().Features.ServiceUnit = "custom-headscale.service"
	for _, test := range []struct {
		cookie *http.Cookie
		source string
		unit   string
	}{{owner, "headdeck", "headdeck.service"}, {admin, "headscale", "custom-headscale.service"}} {
		out := request(t, srv, "GET", "/api/logs/"+test.source+"?unit=sshd.service&lines=999999", "", nil, []*http.Cookie{test.cookie})
		var snapshot protocol.LogSnapshot
		if out.Code != http.StatusOK || json.Unmarshal(out.Body.Bytes(), &snapshot) != nil || len(snapshot.Entries) != 5 {
			t.Fatal("invalid log snapshot", out.Code, out.Body.String())
		}
		if snapshot.Source != test.source || snapshot.CollectedAt.IsZero() || out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid log metadata")
		}
		if snapshot.Entries[0].Level != "error" || strings.Contains(snapshot.Entries[0].Message, "hskey-secret") || snapshot.Entries[1].Level != "warn" || snapshot.Entries[2].Level != "debug" || snapshot.Entries[2].Message != "debug message" || snapshot.Entries[3].Message != "binary" || snapshot.Entries[4].Level != "error" {
			t.Fatalf("log levels or sanitization failed: %+v", snapshot.Entries)
		}
		args, err := os.ReadFile(arguments)
		if err != nil || !strings.Contains(string(args), "--unit="+test.unit+"\n") || !strings.Contains(string(args), "--lines=200\n") || strings.Contains(string(args), "sshd") {
			t.Fatal("journal arguments escaped scope", string(args), err)
		}
	}
	if err := os.WriteFile(data, []byte("invalid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := request(t, srv, "GET", "/api/logs/headdeck", "", nil, []*http.Cookie{owner}); out.Code != http.StatusInternalServerError {
		t.Fatal("invalid journal data accepted", out.Code)
	}
	if err := os.WriteFile(data, []byte(strings.Repeat("x", (2<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := request(t, srv, "GET", "/api/logs/headdeck", "", nil, []*http.Cookie{owner}); out.Code != http.StatusServiceUnavailable || out.Body.Len() > 1000 {
		t.Fatal("oversized log output accepted", out.Code, out.Body.Len())
	}
	if err := os.Remove(filepath.Join(dir, "journalctl")); err != nil {
		t.Fatal(err)
	}
	if out := request(t, srv, "GET", "/api/logs/headdeck", "", nil, []*http.Cookie{owner}); out.Code != http.StatusServiceUnavailable {
		t.Fatal("missing journal reader not reported", out.Code)
	}
}

func TestHeaddeckDebugRequiresDiagnosticOptIn(t *testing.T) {
	srv, _, _ := testWebServer(t)
	owner := login(t, srv)
	member := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, "member", principal.RoleMember, srv.now())}
	admin := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, "administrator", principal.RoleAdmin, srv.now())}
	for _, cookie := range []*http.Cookie{nil, member, owner} {
		var cookies []*http.Cookie
		want := http.StatusUnauthorized
		if cookie != nil {
			cookies = []*http.Cookie{cookie}
			want = http.StatusForbidden
		}
		for _, path := range []string{"/api/debug/pprof/", "/api/debug/pprof/heap", "/api/debug/pprof/profile?seconds=1", "/api/debug/pprof/trace?seconds=1"} {
			if out := request(t, srv, "GET", path, "", nil, cookies); out.Code != want {
				t.Fatal("debug access bypass", path, out.Code)
			}
		}
	}
	out := request(t, srv, "PATCH", "/api/settings", `{"changes":{"server.diagnostics":"true"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{admin})
	if out.Code != http.StatusOK {
		t.Fatal("cannot enable diagnostics", out.Body.String())
	}
	if out := request(t, srv, "GET", "/api/debug/pprof/heap", "", nil, []*http.Cookie{member}); out.Code != http.StatusForbidden {
		t.Fatal("member accessed enabled debug profiles")
	}
	for _, path := range []string{"/api/debug/pprof/", "/api/debug/pprof/goroutine?debug=1", "/api/debug/pprof/heap", "/api/debug/pprof/profile?seconds=1", "/api/debug/pprof/trace?seconds=1"} {
		out := request(t, srv, "GET", path, "", nil, []*http.Cookie{admin})
		if out.Code != http.StatusOK || out.Body.Len() == 0 || out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid debug response", path, out.Code)
		}
	}
	for _, duration := range []string{"0", "-1", "31", "1.5", "NaN", "invalid"} {
		for _, profile := range []string{"profile", "trace", "heap"} {
			out := request(t, srv, "GET", "/api/debug/pprof/"+profile+"?seconds="+duration, "", nil, []*http.Cookie{admin})
			if out.Code != http.StatusBadRequest {
				t.Fatal("invalid profile duration accepted", profile, duration, out.Code)
			}
		}
	}
	out = request(t, srv, "PATCH", "/api/settings", `{"changes":{"server.diagnostics":"false"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{owner})
	if out.Code != http.StatusOK {
		t.Fatal("cannot disable debug profiles")
	}
	if out := request(t, srv, "GET", "/api/debug/pprof/heap", "", nil, []*http.Cookie{owner}); out.Code != http.StatusForbidden {
		t.Fatal("debug remained available after opt-out")
	}
}

func TestHeadscaleProxyRoutingAndUpgrade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "" {
			conn, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			if _, err := rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: tailscale-control-protocol\r\n\r\n"); err != nil {
				t.Error(err)
				return
			}
			if err := rw.Flush(); err != nil {
				t.Error(err)
				return
			}
			if _, err := io.Copy(conn, rw); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
			return
		}
		if r.Header.Get("Authorization") != "Bearer client-key" || strings.Contains(r.Header.Get("Cookie"), "headdeck_") || r.Header.Get("X-Forwarded-For") != "127.0.0.1" || r.Host != "headdeck.example" {
			t.Error("proxy leaked credentials or trusted forged headers", r.Header)
		}
		w.Header().Add("Set-Cookie", "headdeck_session=forged; Path=/")
		w.Header().Add("Set-Cookie", "hs_state=ok; Path=/")
		w.WriteHeader(http.StatusCreated)
	}))
	defer upstream.Close()
	srv, _, _ := testWebServer(t, upstream.URL)
	srv.cfg.PublicURL = "https://headdeck.example"
	if err := srv.initHeadscaleProxy(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	for _, path := range []string{"/api/v1/user", "/oidc/callback", "/register/device"} {
		response := request(t, srv, "POST", path, "{}", map[string]string{"Authorization": "Bearer client-key", "X-Forwarded-For": "forged", "Cookie": "headdeck_session=private; hs_state=ok"}, nil)
		if response.Code != http.StatusCreated || strings.Contains(response.Header().Get("Set-Cookie"), "headdeck_") {
			t.Fatal("upstream routing or cookie isolation failed", path, response.Code)
		}
	}
	if response := request(t, srv, "GET", "/api/session", "", nil, nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "authenticated") {
		t.Fatal("Headdeck API was proxied")
	}
	front := httptest.NewServer(srv)
	defer front.Close()
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(front.URL, "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "POST /ts2021 HTTP/1.1\r\nHost: headdeck.example\r\nConnection: Upgrade\r\nUpgrade: tailscale-control-protocol\r\nContent-Length: 0\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil || response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatal("control protocol upgrade failed", err)
	}
	if _, err := io.WriteString(conn, "ping"); err != nil {
		t.Fatal(err)
	}
	var reply [4]byte
	if _, err := io.ReadFull(reader, reply[:]); err != nil || string(reply[:]) != "ping" {
		t.Fatal("control stream failed", err)
	}
	srv.Close()
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("upgraded connection survived proxy shutdown")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("proxy shutdown left an upgraded connection open")
	}
	srv.authorizeLogin = func(context.Context) error { return errors.New("takeover incomplete") }
	if response := request(t, srv, "GET", "/register/device", "", nil, nil); response.Code != http.StatusServiceUnavailable {
		t.Fatal("proxy bypassed incomplete login takeover")
	}
}

func TestWebRequiresHeadscaleTakeover(t *testing.T) {
	srv, _, _ := testWebServer(t)
	cookie := login(t, srv)
	srv.authorizeLogin = func(context.Context) error { return errors.New("issuer mismatch") }
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/auth/methods"}, {"POST", "/api/auth/password"},
		{"GET", "/api/auth/github/start"}, {"GET", "/api/auth/oidc/callback"},
		{"POST", "/api/auth/passkey/start"}, {"POST", "/api/auth/totp"},
		{"GET", "/api/session"}, {"POST", "/api/session"},
		{"GET", "/api/nodes"}, {"PATCH", "/api/settings"},
	} {
		response := request(t, srv, route.method, route.path, "{}", map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("takeover bypass: %s %s: %d", route.method, route.path, response.Code)
		}
	}
	response := request(t, srv, "DELETE", "/api/session", "", map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
	if response.Code != http.StatusNoContent {
		t.Fatal("sign out blocked during incomplete takeover")
	}
}

func TestManagementRoutesEnforceSessionPermissions(t *testing.T) {
	srv, _, _ := testWebServer(t)
	pending := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, srv.sessions, "pending", principal.RolePending, srv.now())}
	admin := login(t, srv)
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/status"}, {"GET", "/api/status/stream"}, {"GET", "/api/nodes"}, {"GET", "/api/users"}, {"GET", "/api/routes"},
		{"GET", "/api/headscale/diagnostics/metrics"}, {"GET", "/api/headscale/diagnostics/overview"},
		{"GET", "/api/debug/pprof/"}, {"GET", "/api/debug/pprof/heap"},
		{"GET", "/api/preauth-keys"}, {"GET", "/api/events"}, {"GET", "/api/accounts"}, {"GET", "/api/dns"},
		{"POST", "/api/nodes/1/rename"}, {"POST", "/api/nodes/1/tags"}, {"DELETE", "/api/nodes/1"},
		{"POST", "/api/users"}, {"DELETE", "/api/users/1"}, {"POST", "/api/preauth-keys"},
		{"PUT", "/api/dns"}, {"POST", "/api/policy/objects"}, {"PATCH", "/api/headscale/config"},
		{"POST", "/api/headscale/restart"}, {"PUT", "/api/accounts/example"}, {"PATCH", "/api/settings"},
	} {
		for _, cookie := range []*http.Cookie{nil, pending} {
			var cookies []*http.Cookie
			if cookie != nil {
				cookies = []*http.Cookie{cookie}
			}
			out := request(t, srv, route.method, route.path, "{}", map[string]string{"X-Requested-With": "headdeck"}, cookies)
			expected := http.StatusUnauthorized
			if cookie != nil {
				expected = http.StatusForbidden
			}
			if out.Code != expected {
				t.Fatalf("%s %s: expected %d, got %d", route.method, route.path, expected, out.Code)
			}
		}
		if route.method == http.MethodGet {
			continue
		}
		out := request(t, srv, route.method, route.path, "{}", nil, []*http.Cookie{admin})
		if out.Code != http.StatusForbidden {
			t.Fatal("mutation accepted without CSRF header")
		}
	}
	out := request(t, srv, "POST", "/api/nodes/1/rename", `{"new_name":"valid","extra":true}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{admin})
	if out.Code != http.StatusBadRequest {
		t.Fatal("unknown mutation field accepted")
	}
	srv.cfg.PublicURL = "https://headdeck.example.test"
	if !srv.originAllowed("https://headdeck.example.test") || srv.originAllowed("https://headdeck.example.test.evil") || srv.originAllowed("https://headdeck.example.test/path") {
		t.Fatal("public origin validation is incorrect")
	}
	for _, key := range []string{"features.service_unit", "server.web.session_key_file", "server.web.oidc_server.signing_key_file"} {
		out := request(t, srv, "PATCH", "/api/settings", `{"changes":{"`+key+`":"/etc/security-target"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{admin})
		if out.Code == http.StatusOK {
			t.Fatal("owner changed an installation-only setting", key)
		}
	}
}

func TestProxyIPCannotBeSpoofed(t *testing.T) {
	s := &Server{cfg: config.WebConfig{TrustedProxies: []string{"127.0.0.0/8", "10.0.0.0/8"}}}
	for _, sample := range []struct{ peer, forwarded, want string }{
		{"127.0.0.1:1234", "192.0.2.99, 198.51.100.1", "198.51.100.1"},
		{"127.0.0.1:1234", "192.0.2.99, 198.51.100.1, 10.0.0.1", "198.51.100.1"},
		{"198.51.100.1:1234", "192.0.2.99", "198.51.100.1"},
		{"127.0.0.1:1234", "invalid", "127.0.0.1"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = sample.peer
		r.Header.Set("X-Forwarded-For", sample.forwarded)
		if got := s.clientIP(r); got != sample.want {
			t.Fatalf("client IP = %q, want %q", got, sample.want)
		}
	}
}

func TestLoginLimiterRetention(t *testing.T) {
	l := newAttemptLimiter(2, time.Minute)
	now := time.Now()
	for i := range maxLoginClients {
		if !l.allow(strconv.Itoa(i), now) {
			t.Fatal("client rejected before capacity")
		}
	}
	if l.allow("extra", now) || len(l.attempts) != maxLoginClients {
		t.Fatal("login client storage exceeded capacity")
	}
	if !l.allow("extra", now.Add(time.Minute)) || len(l.attempts) != 1 {
		t.Fatal("expired login clients were not released")
	}
}

func testWebServer(t *testing.T, upstream ...string) (*Server, *service.Service, string) {
	t.Helper()
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "web-token")
	if err := os.WriteFile(tokenPath, []byte(testToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Headscale.BaseURL = "http://127.0.0.1:1"
	if len(upstream) != 0 {
		cfg.Headscale.BaseURL = upstream[0]
		cfg.Headscale.APIKey = "fixture-key"
	}
	cfg.Headscale.Timeout = 300 * time.Millisecond
	cfg.Features.ServiceControlDriver = "none"
	cfg.Server.Web.Enabled = true
	cfg.Server.Web.HeadscaleProxy = false
	cfg.Server.Web.Listen = "127.0.0.1:8088"
	cfg.Server.Web.TokenFile = tokenPath

	db, err := config.Open(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	accountStore, err := accounts.New(db.Database())
	if err != nil {
		t.Fatal(err)
	}
	var headscalePath string
	if len(upstream) > 1 {
		headscalePath = upstream[1]
	}
	svc, err := service.New(service.Options{Config: cfg, Hub: events.NewHub(), Accounts: accountStore, HeadscaleConfigPath: headscalePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	srv, err := New(Options{
		Config:         cfg.Server.Web,
		AuthorizeLogin: func(context.Context) error { return nil },
		Service:        svc,
		Settings:       db,
		Logger:         log.New(log.Options{Level: "error"}),
		SessionKeyFile: filepath.Join(dir, "session-key"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv, svc, tokenPath
}

func request(t *testing.T, srv *Server, method, path, body string, headers map[string]string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = "127.0.0.1:12345"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, srv *Server) *http.Cookie {
	t.Helper()
	rec := request(t, srv, http.MethodPost, "/api/session",
		`{"token":"`+testToken+`"}`,
		map[string]string{"X-Requested-With": "headdeck", "Content-Type": "application/json"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			if !c.HttpOnly {
				t.Fatal("the session cookie must be HttpOnly")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Fatal("the session cookie must be SameSite=Strict")
			}
			if c.Expires.IsZero() || c.MaxAge != int(srv.cfg.SessionTTL/time.Second) || c.Secure != srv.cfg.CookieSecure {
				t.Fatal("session cookie persistence or security settings are incorrect")
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("session response may be cached")
			}
			return c
		}
	}
	t.Fatal("login set no session cookie")
	return nil
}

func TestSessionLifecycle(t *testing.T) {
	srv0, _, _ := testWebServer(t)
	store := srv0.sessions
	now := time.Now()
	first := issueTestSession(t, store, "web", "admin", now)
	second := issueTestSession(t, store, "web", "admin", now)
	if first == second {
		t.Fatal("devices share a session")
	}
	if err := store.revoke(first); err != nil {
		t.Fatal(err)
	}
	restarted, err := newSessionStore(store.db, store.key, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := restarted.verify(first, now); ok {
		t.Fatal("restart revived logout")
	}
	if _, ok := restarted.verify(second, now); !ok {
		t.Fatal("restart lost another device")
	}
	restarted.capacity = 1
	if _, err := restarted.issue("extra", "admin", now, ""); err == nil {
		t.Fatal("session limit ignored")
	}
	if _, ok := restarted.verify(second, now); !ok {
		t.Fatal("capacity invalidated another device")
	}
	if _, ok := restarted.verify(second, now.Add(store.ttl)); ok {
		t.Fatal("expired session accepted")
	}

	srv, _, _ := testWebServer(t)

	rec := request(t, srv, http.MethodGet, "/api/session", "", nil, nil)
	if !strings.Contains(rec.Body.String(), `"authenticated":false`) {
		t.Fatalf("anonymous session check = %s", rec.Body.String())
	}

	cookie := login(t, srv)
	rec = request(t, srv, http.MethodGet, "/api/session", "", nil, []*http.Cookie{cookie})
	if !strings.Contains(rec.Body.String(), `"authenticated":true`) {
		t.Fatalf("authenticated session check = %s", rec.Body.String())
	}

	rec = request(t, srv, http.MethodDelete, "/api/session", "",
		map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", rec.Code)
	}
	// The revoked cookie must stop working immediately.
	rec = request(t, srv, http.MethodGet, "/api/status", "", nil, []*http.Cookie{cookie})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status = %d, want 401", rec.Code)
	}
}

func TestProtectedEndpointsRequireASession(t *testing.T) {
	srv, _, _ := testWebServer(t)
	for _, path := range []string{
		"/api/status", "/api/nodes", "/api/users", "/api/routes", "/api/preauth-keys",
	} {
		rec := request(t, srv, http.MethodGet, path, "", nil, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without a session = %d, want 401", path, rec.Code)
		}
	}
}

func TestTamperedCookieIsRejected(t *testing.T) {
	srv, _, _ := testWebServer(t)
	cookie := login(t, srv)

	tampered := *cookie
	tampered.Value = cookie.Value[:len(cookie.Value)-2] + "zz"
	rec := request(t, srv, http.MethodGet, "/api/status", "", nil, []*http.Cookie{&tampered})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered cookie status = %d, want 401", rec.Code)
	}

	// A cookie signed with another key must not verify either.
	otherServer, _, _ := testWebServer(t)
	other := otherServer.sessions
	forgedCookie := &http.Cookie{Name: sessionCookie, Value: issueTestSession(t, other, "web", "admin", time.Now())}
	rec = request(t, srv, http.MethodGet, "/api/status", "", nil, []*http.Cookie{forgedCookie})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged cookie status = %d, want 401", rec.Code)
	}
}

func TestNonGETRequiresTheRequestedWithHeader(t *testing.T) {
	srv, _, _ := testWebServer(t)
	cookie := login(t, srv)
	// Without the custom header a cross-site form post could reach the API.
	rec := request(t, srv, http.MethodDelete, "/api/session", "", nil, []*http.Cookie{cookie})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a missing X-Requested-With", rec.Code)
	}
}

func TestCrossOriginIsRejected(t *testing.T) {
	srv, _, _ := testWebServer(t)
	cookie := login(t, srv)
	rec := request(t, srv, http.MethodDelete, "/api/session", "",
		map[string]string{"X-Requested-With": "headdeck", "Origin": "https://evil.example"},
		[]*http.Cookie{cookie})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a foreign origin", rec.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv, _, _ := testWebServer(t)
	var last *httptest.ResponseRecorder
	for i := 0; i < maxLoginAttemptsPerMinute+3; i++ {
		last = request(t, srv, http.MethodPost, "/api/session", `{"token":"wrong"}`,
			map[string]string{"X-Requested-With": "headdeck"}, nil)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status after %d attempts = %d, want 429",
			maxLoginAttemptsPerMinute+3, last.Code)
	}
	// A correct token from the same address is still rate limited: the limit
	// protects the endpoint, not the credential.
	rec := request(t, srv, http.MethodPost, "/api/session", `{"token":"`+testToken+`"}`,
		map[string]string{"X-Requested-With": "headdeck"}, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("correct token while limited = %d, want 429", rec.Code)
	}
}

func TestMissingTokenFileFailsClosed(t *testing.T) {
	srv, _, tokenPath := testWebServer(t)
	if err := os.Remove(tokenPath); err != nil {
		t.Fatal(err)
	}
	rec := request(t, srv, http.MethodPost, "/api/session", `{"token":"anything"}`,
		map[string]string{"X-Requested-With": "headdeck"}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when the token file is missing", rec.Code)
	}
}

func TestEventStreamRenewsWriteDeadline(t *testing.T) {
	srv, svc, _ := testWebServer(t)
	cookie := login(t, srv)
	httpServer := httptest.NewUnstartedServer(srv)
	httpServer.Config.WriteTimeout = 30 * time.Millisecond
	httpServer.Start()
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpServer.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status=%d", resp.StatusCode)
	}
	<-time.After(100 * time.Millisecond)
	svc.Hub().Publish(protocol.EventNodesChanged, "1")
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == "event: "+protocol.EventNodesChanged {
			return
		}
	}
	t.Fatalf("stream did not outlive Server.WriteTimeout: %v", scanner.Err())
}

func issueTestSession(t *testing.T, s *sessionStore, subject string, role principal.Role, now time.Time) string {
	t.Helper()
	value, err := s.issue(subject, role, now, "")
	if err != nil {
		t.Fatal(err)
	}
	return value
}
