package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"golang.org/x/oauth2"
)

type fixtureProvider struct {
	request   accounts.LoginRequest
	exchanges int
}

func TestRegisterNodeOwnership(t *testing.T) {
	var registered atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/user":
			_, _ = w.Write([]byte(`{"users":[{"id":"1","name":"alice"},{"id":"2","name":"bob"}]}`))
		case "/api/v1/node/register":
			var in struct{ User, Key string }
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Key != "hskey-authreq-test" || (in.User != "alice" && in.User != "bob") {
				t.Error("registration did not resolve the requested owner", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			registered.Add(1)
			id := "1"
			if in.User == "bob" {
				id = "2"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"node": map[string]any{"id": "42", "user": map[string]string{"id": id, "name": in.User}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	srv, svc, _ := testWebServer(t, upstream.URL)
	ctx := context.Background()
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "alice"}, Username: "alice"}
	a, err := svc.LoginAccount(ctx, identity, true)
	if err != nil {
		t.Fatal(err)
	}
	value, err := srv.issueAccount(a, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: value}
	headers := map[string]string{"X-Requested-With": "headdeck"}
	if response := request(t, srv, "POST", "/api/nodes/register", `{"key":"hskey-authreq-test"}`, headers, nil); response.Code != http.StatusUnauthorized {
		t.Fatal("registration accepted an anonymous request", response.Code)
	}
	if response := request(t, srv, "POST", "/api/nodes/register", `{"key":"hskey-authreq-test"}`, headers, []*http.Cookie{cookie}); response.Code != http.StatusForbidden {
		t.Fatal("registration accepted a pending account", response.Code)
	}
	a.Role, a.HeadscaleUserID = principal.RoleMember, "1"
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"key":"hskey-authreq-test"}`, `{"key":"hskey-authreq-test","user_id":"1"}`} {
		response := request(t, srv, "POST", "/api/nodes/register", body, headers, []*http.Cookie{cookie})
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"alice"`) {
			t.Fatal("member registration did not use its bound user", response.Code, response.Body.String())
		}
	}
	if registered.Load() != 2 {
		t.Fatal("unexpected registration count", registered.Load())
	}
	response := request(t, srv, "POST", "/api/nodes/register", `{"key":"hskey-authreq-test","user_id":"2"}`, headers, []*http.Cookie{cookie})
	if response.Code != http.StatusForbidden || registered.Load() != 2 {
		t.Fatal("member registered to another user", response.Code)
	}
	admin := login(t, srv)
	for _, invalid := range []string{`{"key":"","user_id":"2"}`, `{"key":"https://control.example/register/key","user_id":"2"}`, `{"key":"hskey-authreq-test","user_id":"99"}`} {
		response := request(t, srv, "POST", "/api/nodes/register", invalid, headers, []*http.Cookie{admin})
		if response.Code != http.StatusBadRequest || registered.Load() != 2 {
			t.Fatal("invalid registration reached Headscale", response.Code)
		}
	}
	response = request(t, srv, "POST", "/api/nodes/register", `{"key":"hskey-authreq-test","user_id":"2"}`, headers, []*http.Cookie{admin})
	if response.Code != http.StatusOK || registered.Load() != 3 || !strings.Contains(response.Body.String(), `"name":"bob"`) {
		t.Fatal("administrator could not choose the device owner", response.Code, response.Body.String())
	}
}

func (p *fixtureProvider) AuthorizationURL(_ context.Context, req accounts.LoginRequest) (string, error) {
	p.request = req
	return "https://login.example/authorize?state=" + url.QueryEscape(req.State), nil
}

func (p *fixtureProvider) Exchange(_ context.Context, callback accounts.Callback) (accounts.VerifiedIdentity, error) {
	p.exchanges++
	if callback.Nonce != p.request.Nonce || callback.RedirectURI != p.request.RedirectURI || oauth2.S256ChallengeFromVerifier(callback.CodeVerifier) != p.request.CodeChallenge {
		return accounts.VerifiedIdentity{}, errors.New("login protection lost")
	}
	return accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "new-user"}, Name: "汐殇", Username: "xishang0128"}, nil
}

func TestProviderFlowBoundToBrowserAndUsedOnce(t *testing.T) {
	for _, name := range []string{"oidc", "github"} {
		for _, prefix := range []string{"", "/headdeck"} {
			t.Run(name+prefix, func(t *testing.T) {
				srv, _, _ := testWebServer(t)
				provider := &fixtureProvider{}
				srv.providers[name] = provider
				srv.cfg.OIDC.AutoRegister = true
				srv.cfg.GitHub.AutoRegister = true
				srv.cfg.PublicURL = "https://headdeck.example" + prefix
				start := request(t, srv, "GET", prefix+"/api/auth/"+name+"/start", "", nil, nil)
				if start.Code != http.StatusFound || len(start.Result().Cookies()) != 1 {
					t.Fatal("login flow did not start")
				}
				flow := start.Result().Cookies()[0]
				if flow.Path != prefix+"/api/auth/"+name+"/callback" || provider.request.RedirectURI != srv.cfg.PublicURL+"/api/auth/"+name+"/callback" {
					t.Fatal("wrong callback scope")
				}
				path := prefix + "/api/auth/" + name + "/callback" + "?code=fixture&state=" + provider.request.State
				wrongBrowser := &http.Cookie{Name: flow.Name, Value: "wrong-browser"}
				wrong := request(t, srv, "GET", path, "", nil, []*http.Cookie{wrongBrowser})
				if wrong.Header().Get("Location") != prefix+"/?login_error=invalid_flow" || provider.exchanges != 0 {
					t.Fatal("flow was not bound to its browser")
				}
				callback := request(t, srv, "GET", path, "", nil, []*http.Cookie{flow})
				if callback.Header().Get("Location") != prefix+"/" || provider.exchanges != 1 {
					t.Fatal("verified login did not succeed")
				}
				var cookie *http.Cookie
				for _, c := range callback.Result().Cookies() {
					if c.Name == sessionCookie {
						cookie = c
					}
				}
				if cookie == nil || !cookie.HttpOnly || cookie.Path != prefix+"/" {
					t.Fatal("login did not issue a protected session")
				}
				session := request(t, srv, "GET", prefix+"/api/session", "", nil, []*http.Cookie{cookie})
				var body struct {
					Authenticated bool     `json:"authenticated"`
					Role          string   `json:"role"`
					Permissions   []string `json:"permissions"`
				}
				if err := json.Unmarshal(session.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if !body.Authenticated || body.Role != "pending" || len(body.Permissions) != 0 {
					t.Fatal("new login did not wait for approval")
				}
				account, err := srv.credentials.Resolve(context.Background(), accounts.Identity{Issuer: "https://login.example", Subject: "new-user"})
				if err != nil || account.Username != "xishang0128" || account.Label != "汐殇" {
					t.Fatalf("new login account = %+v, error = %v", account, err)
				}
				replay := request(t, srv, "GET", path, "", nil, []*http.Cookie{flow})
				if replay.Header().Get("Location") != prefix+"/?login_error=invalid_flow" || provider.exchanges != 1 {
					t.Fatal("callback was replayable")
				}

			})
		}
	}
}

func TestAccountApprovalScopeAndRevocation(t *testing.T) {
	var mutations atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/api/v1/user":
			body = `{"users":[{"id":"1","name":"alice"},{"id":"2","name":"bob"}]}`
		case "/api/v1/node":
			body = `{"nodes":[{"id":"10","givenName":"alice-laptop","user":{"id":"1","name":"alice"}},{"id":"20","givenName":"bob-server","user":{"id":"2","name":"bob"}}]}`
		case "/api/v1/node/10", "/api/v1/node/10/rename/alice-phone":
			body = `{"node":{"id":"10","givenName":"alice-phone","user":{"id":"1","name":"alice"}}}`
		case "/api/v1/node/20":
			body = `{"node":{"id":"20","givenName":"bob-server","user":{"id":"2","name":"bob"}}}`
		case "/api/v1/preauthkey/expire":
			body = `{}`
		case "/api/v1/preauthkey":
			body = `{"preAuthKeys":[{"id":"10","user":{"id":"1","name":"alice"}},{"id":"20","user":{"id":"2","name":"bob"}}]}`
		default:
			http.NotFound(w, r)
		}
		if body != "" {
			if _, err := w.Write([]byte(body)); err != nil {
				t.Error(err)
			}
		}
	}))
	defer upstream.Close()
	srv, svc, _ := testWebServer(t, upstream.URL)
	ctx := context.Background()
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "alice"}, Name: "Alice", AvatarURL: "https://avatars.example/alice.png"}
	a, err := svc.LoginAccount(ctx, identity, true)
	if err != nil || a.Role != principal.RolePending {
		t.Fatalf("new account must wait for approval: %+v, %v", a, err)
	}
	value, err := srv.issueAccount(a, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: value}
	get := func(path string) *httptest.ResponseRecorder {
		return request(t, srv, "GET", path, "", nil, []*http.Cookie{cookie})
	}
	if get("/api/session").Code != http.StatusOK || get("/api/nodes").Code != http.StatusForbidden {
		t.Fatal("pending account is not confined to its session")
	}
	if !strings.Contains(get("/api/session").Body.String(), identity.AvatarURL) {
		t.Fatal("verified avatar was lost from the session")
	}
	for _, invalid := range []string{"javascript:alert(1)", "http://avatars.example/alice.png", "https://user:password@avatars.example/alice.png"} {
		if safeAvatar(invalid) != "" {
			t.Fatal("unsafe avatar URL accepted")
		}
	}
	a.Role = principal.RoleMember
	a.HeadscaleUserID = "1"
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []struct{ path, key string }{
		{"/api/nodes?user=bob", "nodes"}, {"/api/users", "users"}, {"/api/preauth-keys?user=bob", "keys"},
	} {
		response := get(collection.path)
		var body map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var items []map[string]any
		if err := json.Unmarshal(body[collection.key], &items); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(items) != 1 {
			t.Fatalf("member scope %s: %d %s", collection.path, response.Code, response.Body.String())
		}
		owner := items[0]["id"]
		if user, ok := items[0]["user"].(map[string]any); ok {
			owner = user["id"]
		}
		if owner != "1" {
			t.Fatalf("another user's data exposed: %s", response.Body.String())
		}
	}
	for _, path := range []string{"/api/status", "/api/events", "/api/accounts", "/api/dns", "/api/settings"} {
		if get(path).Code != http.StatusForbidden {
			t.Fatalf("member accessed %s", path)
		}
	}
	for _, action := range []struct{ method, path, body string }{
		{"POST", "/api/nodes/register", `{"user_id":"2","key":"hskey-authreq-test"}`},
		{"POST", "/api/nodes/20/rename", `{"new_name":"stolen"}`},
		{"POST", "/api/nodes/20/expire", ""},
		{"POST", "/api/nodes/10/owner", `{"user_id":"2"}`},
		{"POST", "/api/preauth-keys", `{"user":"2"}`},
		{"POST", "/api/preauth-keys", `{"user":"1","acl_tags":["tag:admin"]}`},
		{"POST", "/api/preauth-keys/20/expire", ""},
		{"DELETE", "/api/preauth-keys/20", ""},
	} {
		response := request(t, srv, action.method, action.path, action.body, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
		if response.Code != http.StatusForbidden || mutations.Load() != 0 {
			t.Fatalf("member mutation escaped scope %s: %d %s", action.path, response.Code, response.Body.String())
		}
	}
	for _, action := range []struct{ path, body string }{
		{"/api/nodes/10/rename", `{"new_name":"alice-phone"}`},
		{"/api/preauth-keys/10/expire", ""},
	} {
		response := request(t, srv, "POST", action.path, action.body, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
		if response.Code != http.StatusOK {
			t.Fatalf("own resource mutation rejected: %d %s", response.Code, response.Body.String())
		}
	}
	if mutations.Load() != 2 {
		t.Fatal("own mutations did not reach Headscale")
	}
	for _, userID := range []string{"2", ""} {
		a.HeadscaleUserID = userID
		if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
			t.Fatal(err)
		}
		if get("/api/nodes").Code != http.StatusUnauthorized {
			t.Fatal("old session survived a Headscale binding change")
		}
		a, err = svc.LoginAccount(ctx, identity, false)
		if err != nil {
			t.Fatal(err)
		}
		cookie.Value, err = srv.issueAccount(a, identity, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	if get("/api/nodes").Code != http.StatusForbidden {
		t.Fatal("unlinked member can still access devices")
	}
	a.HeadscaleUserID = "1"
	a.Role = principal.RoleAdmin
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	if get("/api/settings").Code != http.StatusOK || get("/api/accounts").Code != http.StatusForbidden {
		t.Fatal("approval did not take effect for the existing session")
	}
	reloads := 0
	srv.reload = func() { reloads++ }
	saved := request(t, srv, "PATCH", "/api/settings", `{"changes":{"server.log_level":"warn"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
	if saved.Code != http.StatusOK || reloads != 1 {
		t.Fatal("successful settings write did not automatically reload", saved.Code, reloads)
	}
	invalid := request(t, srv, "PATCH", "/api/settings", `{"changes":{"server.log_json":"invalid"}}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
	if invalid.Code == http.StatusOK || reloads != 1 {
		t.Fatal("failed settings write triggered a reload")
	}
	for _, path := range []string{"/api/accounts", "/api/accounts/" + a.ID + "/security"} {
		out := request(t, srv, http.MethodPost, path, `{}`, map[string]string{"X-Requested-With": "headdeck"}, []*http.Cookie{cookie})
		if out.Code != http.StatusForbidden {
			t.Fatal("administrator can manage other accounts", path, out.Code)
		}
	}
	a.Role = principal.RolePending
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	if get("/api/nodes").Code != http.StatusForbidden {
		t.Fatal("demotion did not take effect")
	}
	a.Disabled = true
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	a.Disabled = false
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	if get("/api/nodes").Code != http.StatusUnauthorized {
		t.Fatal("old session revived after re-enabling the account")
	}
	a, err = svc.LoginAccount(ctx, identity, false)
	if err != nil {
		t.Fatal(err)
	}
	value, err = srv.issueAccount(a, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	cookie.Value = value
	if err := svc.BindIdentity(ctx, a.ID, identity.Identity, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.BindIdentity(ctx, a.ID, identity.Identity, false); err != nil {
		t.Fatal(err)
	}
	if get("/api/nodes").Code != http.StatusUnauthorized {
		t.Fatal("old session revived after rebinding the identity")
	}
}

func TestDeploymentPath(t *testing.T) {
	srv, _, _ := testWebServer(t)
	srv.cfg.PublicURL = "https://headdeck.example/headdeck"
	for _, path := range []string{"/api/session", "/headdeck-other/api/session"} {
		if request(t, srv, "GET", path, "", nil, nil).Code != http.StatusNotFound {
			t.Fatal("request escaped deployment path")
		}
	}
	redirect := request(t, srv, "GET", "/headdeck?x=1", "", nil, nil)
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/headdeck/?x=1" {
		t.Fatal("invalid path redirect")
	}
	index := request(t, srv, "GET", "/headdeck/", "", nil, nil)
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), `<base href="/headdeck/">`) {
		t.Fatal("missing deployment base")
	}
	if !srv.originAllowed("https://headdeck.example") || srv.originAllowed("https://other.example") {
		t.Fatal("invalid deployment origin")
	}
}

func TestLocalLoginUsesAccountAvatar(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		identity accounts.VerifiedIdentity
		avatar   string
	}{
		{"oauth", accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "alice"}, AvatarURL: "https://avatars.example/alice.png"}, "https://avatars.example/alice.png"},
		{"existing-github", accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://github.com", Subject: "95160953"}}, "https://avatars.githubusercontent.com/u/95160953"},
		{"invalid-github-subject", accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://github.com", Subject: "../someone"}}, ""},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			srv, svc, _ := testWebServer(t)
			ctx := context.Background()
			a, err := svc.LoginAccount(ctx, fixture.identity, true)
			if err != nil {
				t.Fatal(err)
			}
			// Passkey and password sign-ins both issue a local identity session.
			local := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: accounts.LocalIssuer, Subject: a.ID}}
			a, err = svc.LoginAccount(ctx, local, false)
			if err != nil {
				t.Fatal(err)
			}
			value, err := srv.issueAccount(a, local, "")
			if err != nil {
				t.Fatal(err)
			}
			cookie := &http.Cookie{Name: sessionCookie, Value: value}
			assertAvatar := func() {
				t.Helper()
				out := request(t, srv, "GET", "/api/session", "", nil, []*http.Cookie{cookie})
				var body struct {
					AvatarURL string `json:"avatar_url"`
				}
				if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil || out.Code != http.StatusOK || body.AvatarURL != fixture.avatar {
					t.Fatalf("local session lost account avatar: %d %s (%v)", out.Code, out.Body.String(), err)
				}
			}
			assertAvatar()
			other, err := svc.LoginAccount(ctx, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://other.example", Subject: fixture.identity.Subject}}, true)
			if err != nil || other.ID == a.ID || other.AvatarURL != "" {
				t.Fatalf("avatar crossed account boundary: %+v (%v)", other, err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/account/passkeys/finish", nil)
			req.AddCookie(cookie)
			out := httptest.NewRecorder()
			if err := srv.renewAccount(out, req, a.ID); err != nil {
				t.Fatal(err)
			}
			cookie = out.Result().Cookies()[0]
			assertAvatar()
			if err := srv.credentials.SetAvatar(ctx, a.ID, "javascript:alert(1)"); err != nil {
				t.Fatal(err)
			}
			assertAvatar()
		})
	}
}
