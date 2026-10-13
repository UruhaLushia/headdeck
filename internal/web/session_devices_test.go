package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/config"
)

func TestSessionRenewalKeepsDeviceAndAuthenticationTime(t *testing.T) {
	srv, svc, _ := testWebServer(t)
	const lifetime = 7 * 24 * time.Hour
	if srv.sessions.ttl != lifetime {
		t.Fatalf("default session duration = %s", srv.sessions.ttl)
	}
	started := time.Now().Truncate(time.Second)
	now := started
	srv.now = func() time.Time { return now }
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "alice"}}
	a, err := svc.LoginAccount(context.Background(), identity, true)
	if err != nil {
		t.Fatal(err)
	}
	value, err := srv.issueAccount(a, identity, "Chrome/141")
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: value}
	initial, ok := srv.sessions.verify(value, now)
	if !ok {
		t.Fatal("new session invalid")
	}
	if out := request(t, srv, "GET", "/api/session", "", nil, []*http.Cookie{cookie}); len(out.Result().Cookies()) != 0 {
		t.Fatal("fresh session was renewed")
	}
	now = started.Add(4 * 24 * time.Hour)
	for _, test := range []struct {
		method, path, header, value string
	}{
		{"GET", "/api/events", "", ""},
		{"GET", "/api/status/stream", "", ""},
		{"GET", "/api/session", "Sec-Fetch-Site", "cross-site"},
		{"GET", "/api/session", "Origin", "https://other.example"},
		{"DELETE", "/api/session", "", ""},
	} {
		req := httptest.NewRequest(test.method, test.path, nil)
		req.AddCookie(cookie)
		if test.header != "" {
			req.Header.Set(test.header, test.value)
		}
		out := httptest.NewRecorder()
		srv.maybeRenewSession(out, req, initial)
		if len(out.Result().Cookies()) != 0 {
			t.Fatalf("background or cross-site request renewed session: %+v", test)
		}
	}
	results := make(chan *httptest.ResponseRecorder, 8)
	var requests sync.WaitGroup
	for range 8 {
		requests.Go(func() {
			results <- request(t, srv, "GET", "/api/session", "", nil, []*http.Cookie{cookie})
		})
	}
	requests.Wait()
	close(results)
	renewals := 0
	for out := range results {
		if out.Code != http.StatusOK {
			t.Fatalf("concurrent session request failed: %d %s", out.Code, out.Body.String())
		}
		for _, renewed := range out.Result().Cookies() {
			renewals++
			if renewed.Value != value || renewed.MaxAge != int(lifetime/time.Second) || !renewed.Expires.Equal(now.Add(lifetime)) || !renewed.HttpOnly || renewed.SameSite != http.SameSiteLaxMode {
				t.Fatalf("unexpected renewed cookie: %+v", renewed)
			}
		}
	}
	if renewals != 1 {
		t.Fatalf("concurrent requests renewed cookie %d times", renewals)
	}
	list := request(t, srv, "GET", "/api/sessions", "", nil, []*http.Cookie{cookie})
	var devices []loginDevice
	if err := json.Unmarshal(list.Body.Bytes(), &devices); err != nil || len(devices) != 1 || devices[0].ID != sessionHash(value) || !devices[0].CreatedAt.Equal(started) || !devices[0].ExpiresAt.Equal(now.Add(lifetime)) {
		t.Fatalf("renewal changed login device: %s (%v)", list.Body.String(), err)
	}
	req := httptest.NewRequest(http.MethodGet, "/oidc/login", nil)
	req.AddCookie(cookie)
	profile, err := srv.oidcSession(req)
	if err != nil || !profile.AuthenticatedAt.Equal(started) {
		t.Fatalf("renewal changed OIDC authentication time: %+v (%v)", profile, err)
	}
	restarted, err := newSessionStore(srv.sessions.db, srv.sessions.key, lifetime)
	if err != nil {
		t.Fatal(err)
	}
	now = started.Add(8 * 24 * time.Hour)
	current, ok := restarted.verify(value, now)
	if !ok || !current.CreatedAt.Equal(started) {
		t.Fatal("renewed cookie did not survive original expiry and restart")
	}
	if _, ok := restarted.verify(value, current.ExpiresAt); ok {
		t.Fatal("idle session did not expire")
	}
	if renewed, err := restarted.renew(value, current, current.ExpiresAt); err != nil || renewed {
		t.Fatalf("renewal revived expired session: %v %v", renewed, err)
	}
	if err := restarted.revoke(value); err != nil {
		t.Fatal(err)
	}
	if renewed, err := restarted.renew(value, current, now); err != nil || renewed {
		t.Fatalf("renewal revived revoked session: %v %v", renewed, err)
	}
	if _, ok := restarted.verify(value, now); ok {
		t.Fatal("revoked session still valid")
	}
}

func TestLoginDeviceDetails(t *testing.T) {
	for _, test := range []struct {
		agent, browser, browserVersion, platform, platformVersion string
	}{
		{"Mozilla/5.0 (Linux; Android 15) AppleWebKit/537.36 Chrome/140.0.0.0 Mobile Safari/537.36", "Chrome", "140.0.0.0", "Android", "15"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 Version/18.6 Mobile/15E148 Safari/604.1", "Safari", "18.6", "iOS", "18.6"},
		{"Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X) AppleWebKit/605.1.15 CriOS/140.0.7339.39 Mobile/15E148 Safari/604.1", "Chrome", "140.0.7339.39", "iOS", "18.6"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/140.0.0.0 Safari/537.36 Edg/140.0.3485.54", "Edge", "140.0.3485.54", "Windows", ""},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15", "Safari", "18.0", "macOS", "14.6"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:143.0) Gecko/20100101 Firefox/143.0", "Firefox", "143.0", "Linux", ""},
		{"", "Browser", "", "", ""},
	} {
		t.Run(test.browser+test.platform, func(t *testing.T) {
			device := parseLoginDevice(test.agent)
			if device.Browser != test.browser || device.BrowserVersion != test.browserVersion || device.Platform != test.platform || device.PlatformVersion != test.platformVersion {
				t.Fatalf("device details = %+v", device)
			}
		})
	}
}

func TestSessionsArePersistentAndAccountScoped(t *testing.T) {
	srv, svc, _ := testWebServer(t)
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "alice"}, Name: "Alice"}
	account, err := svc.LoginAccount(context.Background(), identity, true)
	if err != nil {
		t.Fatal(err)
	}
	first, err := srv.issueAccount(account, identity, "Chrome/120")
	if err != nil {
		t.Fatal(err)
	}
	second, err := srv.issueAccount(account, identity, "Firefox/130")
	if err != nil {
		t.Fatal(err)
	}
	identity.Subject = "bob"
	otherAccount, err := svc.LoginAccount(context.Background(), identity, true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := srv.issueAccount(otherAccount, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := newSessionStore(srv.sessions.db, srv.sessions.key, srv.sessions.ttl)
	if err != nil {
		t.Fatal(err)
	}
	srv.sessions = restarted
	cookie := []*http.Cookie{{Name: sessionCookie, Value: first}}
	list := request(t, srv, "GET", "/api/sessions", "", nil, cookie)
	var devices []loginDevice
	if err := json.Unmarshal(list.Body.Bytes(), &devices); err != nil {
		t.Fatal(err)
	}
	if list.Code != 200 || len(devices) != 2 {
		t.Fatal("sessions were lost or crossed accounts")
	}
	current := 0
	for _, device := range devices {
		if device.Browser == "Chrome" && device.BrowserVersion != "120" || device.Browser == "Firefox" && device.BrowserVersion != "130" {
			t.Fatalf("stored session lost browser version: %+v", device)
		}
		if device.Current {
			current++
		}
	}
	if current != 1 {
		t.Fatal("current device not identified")
	}
	headers := map[string]string{"X-Requested-With": "headdeck"}
	if request(t, srv, "DELETE", "/api/sessions/"+sessionHash(other), "", headers, cookie).Code != 404 {
		t.Fatal("other account session could be revoked")
	}
	if request(t, srv, "DELETE", "/api/sessions/"+sessionHash(second), "", headers, cookie).Code != 204 {
		t.Fatal("device logout failed")
	}
	if _, ok := srv.sessions.verify(first, time.Now()); !ok {
		t.Fatal("device logout affected current browser")
	}
	if _, ok := srv.sessions.verify(second, time.Now()); ok {
		t.Fatal("device logout failed to revoke cookie")
	}
	if request(t, srv, "DELETE", "/api/sessions", "", headers, cookie).Code != 204 {
		t.Fatal("account logout failed")
	}
	var sequence int
	var name, path string
	if err := srv.sessions.db.QueryRow("PRAGMA database_list").Scan(&sequence, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := srv.sessions.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := config.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	restarted, err = newSessionStore(db.Database(), srv.sessions.key, srv.sessions.ttl)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := restarted.verify(first, time.Now()); ok {
		t.Fatal("restart revived account logout")
	}
	if _, ok := restarted.verify(other, time.Now()); !ok {
		t.Fatal("account logout affected another account")
	}
	rotated, err := newSessionStore(db.Database(), []byte("a different session signing key"), srv.sessions.ttl)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rotated.verify(other, time.Now()); ok {
		t.Fatal("key rotation retained an old session")
	}
}
