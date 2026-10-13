package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/github"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/config"
)

func TestOIDCHeadscaleBinding(t *testing.T) {
	ctx := context.Background()
	var user atomic.Value
	user.Store(map[string]string{"id": "10", "name": "xishang0128", "email": "owner@example.com", "providerId": "https://old.example/subject"})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"users": []any{user.Load()}}); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	srv, svc, _ := testWebServer(t, upstream.URL)
	srv.cfg.PublicURL = "https://headdeck.example/admin"
	srv.cfg.CookieSecure = true
	srv.cfg.OIDCServer = config.OIDCServerConfig{Enabled: true, ClientID: "headscale", ClientSecret: strings.Repeat("s", 32), RedirectURI: "https://headscale.example/oidc/callback", SigningKeyFile: filepath.Join(t.TempDir(), "key.pem"), TokenTTL: time.Hour}
	if err := srv.oidcRoutes(srv.mux); err != nil {
		t.Fatal(err)
	}
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: github.Issuer, Subject: "95160953"}, Name: "汐殇", Username: "xishang0128", VerifiedEmails: []string{"owner@example.com"}}
	a, err := svc.LoginAccount(ctx, identity, true)
	if err != nil {
		t.Fatal(err)
	}
	value, err := srv.issueAccount(a, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	session := &http.Cookie{Name: sessionCookie, Value: value}
	query := url.Values{"client_id": {"headscale"}, "redirect_uri": {srv.cfg.OIDCServer.RedirectURI}, "response_type": {"code"}, "scope": {"openid profile email"}, "state": {"state"}, "nonce": {"nonce"}}
	start := request(t, srv, "GET", "/admin/oidc/authorize?"+query.Encode(), "", nil, nil)
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil || location.Query().Get("id") == "" {
		t.Fatalf("start: %d %s", start.Code, start.Body.String())
	}
	loginURL := location.RequestURI()
	pending := request(t, srv, "GET", loginURL, "", nil, []*http.Cookie{session})
	if pending.Header().Get("Location") != "/admin/" {
		t.Fatalf("pending bypass: %d %s", pending.Code, pending.Header().Get("Location"))
	}
	var flow *http.Cookie
	for _, cookie := range pending.Result().Cookies() {
		if cookie.Name == oidcFlowCookie {
			flow = cookie
		}
	}
	if flow == nil || !flow.HttpOnly || !flow.Secure || flow.SameSite != http.SameSiteLaxMode {
		t.Fatal("missing protected continuation")
	}
	a.Role = policy.RoleMember
	a.HeadscaleUserID = "10"
	a, err = svc.SaveAccount(ctx, a.ID, *a)
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, srv, "GET", loginURL, "", nil, []*http.Cookie{session})
	if response.Code != http.StatusForbidden {
		t.Fatal("old issuer silently created a duplicate Headscale user")
	}
	record := map[string]string{"id": "10", "name": "xishang0128", "email": "owner@example.com", "providerId": srv.cfg.OIDCIssuer() + "/" + a.ID}
	user.Store(record)
	otherEmail, err := srv.oidcProfile(ctx, a, accountSession{Identity: identity.Identity, VerifiedEmails: []string{"someone@example.com"}})
	if err != nil || otherEmail.EmailVerified || otherEmail.Email != record["email"] {
		t.Fatal("explicit identity binding was rejected or its email was overwritten", err)
	}
	response = request(t, srv, "GET", "/admin/api/session", "", nil, []*http.Cookie{session, flow})
	if !strings.Contains(response.Body.String(), "continue_url") || !strings.Contains(response.Body.String(), "/admin/oidc/login") {
		t.Fatal("approval did not resume login")
	}
	if !strings.Contains(response.Body.String(), `"login_server":"https://headscale.example"`) {
		t.Fatal("device command must use Headscale's public URL, not the Web path or internal API address")
	}
	response = request(t, srv, "GET", loginURL, "", nil, []*http.Cookie{session})
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "/admin/oidc/authorize/callback") {
		t.Fatalf("bound login: %d %s", response.Code, response.Body.String())
	}
	verified, err := srv.oidcSession(httptest.NewRequest("GET", "/", nil))
	if err == nil || verified.Identity.Issuer != "" {
		t.Fatal("missing account session accepted")
	}
	profile, err := srv.oidcProfile(ctx, a, accountSession{Identity: identity.Identity, VerifiedEmails: identity.VerifiedEmails, AuthenticatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.validateOIDCProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	local, err := srv.oidcProfile(ctx, a, accountSession{Identity: accounts.Identity{Issuer: accounts.LocalIssuer, Subject: a.ID}, AuthenticatedAt: time.Now()})
	if err != nil || local.EmailVerified || local.Email != record["email"] {
		t.Fatal("local login fabricated verified email or changed the bound profile", err)
	}
	if err := srv.validateOIDCProfile(ctx, local); err != nil {
		t.Fatal("bound local login rejected", err)
	}
	anonymous := request(t, srv, "GET", loginURL, "", nil, nil)
	if anonymous.Header().Get("Location") != "/admin/?sign_in=1" {
		t.Fatal("Headscale login bypassed Headdeck login selection")
	}
	a.Disabled = true
	if _, err := svc.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	if err := srv.validateOIDCProfile(ctx, profile); err == nil {
		t.Fatal("disabled account retained Headscale login")
	}
}

func TestOIDCAutomaticRegistration(t *testing.T) {
	ctx := context.Background()
	var users atomic.Value
	users.Store([]map[string]string{{"id": "10", "name": "alice", "provider": "oidc", "providerId": "https://old.example/other"}})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/user" {
			t.Error("Web signup attempted to create a Headscale user")
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"users": users.Load()}); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	srv, svc, _ := testWebServer(t, upstream.URL)
	srv.cfg.PublicURL = "https://headdeck.example"
	srv.cfg.DefaultRole = "member"
	srv.cfg.OIDCServer.Enabled = true
	srv.cfg.OIDCServer.RedirectURI = "https://headdeck.example/oidc/callback"
	svc.Config().Server.Web = srv.cfg
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: github.Issuer, Subject: "123"}, Username: "alice", Name: "Alice", VerifiedEmails: []string{"shared@example.test"}}
	a, err := svc.LoginAccount(ctx, identity, true)
	if err != nil || a.Role != policy.RoleMember || a.HeadscaleUserID != "" {
		t.Fatalf("signup = %+v, error = %v", a, err)
	}
	value, err := srv.issueAccount(a, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, srv, "GET", "/api/session", "", nil, []*http.Cookie{{Name: sessionCookie, Value: value}})
	if !strings.Contains(response.Body.String(), `"device_registration":true`) || !strings.Contains(response.Body.String(), `"login_server":"https://headdeck.example"`) {
		t.Fatal("public member cannot obtain the first-device command", response.Body.String())
	}
	profile, err := srv.oidcProfile(ctx, a, accountSession{Identity: identity.Identity, VerifiedEmails: identity.VerifiedEmails})
	if err != nil || profile.HeadscaleUserID != "" || profile.Username == "alice" || profile.AccountID != a.ID {
		t.Fatalf("registration profile = %+v, error = %v", profile, err)
	}
	if err := srv.validateOIDCProfile(ctx, profile); err != nil {
		t.Fatal("device registration denied", err)
	}
	users.Store([]map[string]string{{"id": "11", "name": "wrong-user", "provider": "oidc", "providerId": srv.cfg.OIDCIssuer() + "/" + a.ID}})
	if err := srv.validateOIDCProfile(ctx, profile); err == nil {
		t.Fatal("mismatched upstream username accepted")
	}
	if _, err := svc.UserSnapshot(ctx); err == nil {
		t.Fatal("mismatched upstream username automatically bound")
	}
	users.Store([]map[string]string{{"id": "11", "name": profile.Username, "email": profile.Email, "provider": "oidc", "providerId": srv.cfg.OIDCIssuer() + "/" + a.ID}})
	if _, err := svc.UserSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	bound, err := srv.credentials.Account(ctx, a.ID)
	if err != nil || bound.HeadscaleUserID != "11" || bound.Username != profile.Username || bound.SecurityVersion != a.SecurityVersion {
		t.Fatalf("binding = %+v, error = %v", bound, err)
	}
	if err := srv.validateOIDCProfile(ctx, profile); err != nil {
		t.Fatal("registration token invalidated before Headscale finished", err)
	}
	srv.cfg.DefaultRole = "pending"
	svc.Config().Server.Web.DefaultRole = "pending"
	identity.Subject = "456"
	pending, err := svc.LoginAccount(ctx, identity, true)
	if err != nil || pending.Role != policy.RolePending || pending.HeadscaleUserID != "" {
		t.Fatalf("private signup = %+v, error = %v", pending, err)
	}
	if _, err := srv.oidcProfile(ctx, pending, accountSession{Identity: identity.Identity}); err == nil {
		t.Fatal("pending account registered a device")
	}
}
