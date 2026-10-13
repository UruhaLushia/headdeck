package web

import (
	"context"
	"encoding/json"
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/pquerna/otp/totp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func namedCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name && cookie.MaxAge >= 0 {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie", name)
	return nil
}
func TestPasswordSecondFactorAndRevocation(t *testing.T) {
	srv, _, _ := testWebServer(t)
	ctx := context.Background()
	headers := map[string]string{"X-Requested-With": "headdeck"}
	call := func(method, path, body string, cookies []*http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		out := request(t, srv, method, path, body, headers, cookies)
		if out.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, out.Code, out.Body.String())
		}
		return out
	}
	admin := login(t, srv)
	call("POST", "/api/accounts", `{"label":"bad","role":"pending","username":"bad","password":"short"}`, []*http.Cookie{admin}, 400)
	list, err := srv.credentials.List(ctx)
	if err != nil || len(list) != 0 {
		t.Fatal("failed creation left an account")
	}
	created := call("POST", "/api/accounts", `{"label":"Alice","role":"pending","username":"Alice","password":"initial-password-123"}`, []*http.Cookie{admin}, 200)
	var a accounts.Account
	if err = json.Unmarshal(created.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Username != "alice" || strings.Contains(created.Body.String(), "initial-password") {
		t.Fatal("unsafe credentials response")
	}
	call("POST", "/api/auth/password", `{"username":"alice","password":"initial-password-123"}`, nil, 403)
	methods := call("GET", "/api/auth/methods", "", nil, 200)
	if !strings.Contains(methods.Body.String(), `"password":false`) {
		t.Fatal("disabled password login advertised")
	}
	srv.cfg.PasswordLogin = true
	call("POST", "/api/auth/password", `{"username":"alice","password":"wrong"}`, nil, 401)
	first := call("POST", "/api/auth/password", `{"username":"ALICE","password":"initial-password-123"}`, nil, 200)
	old := namedCookie(t, first, sessionCookie)
	call("GET", "/api/users", "", []*http.Cookie{old}, 403)
	call("GET", "/api/account/security", "", []*http.Cookie{old}, 200)
	call("PUT", "/api/account/profile", `{"label":"Alice updated","role":"admin"}`, []*http.Cookie{old}, 400)
	call("PUT", "/api/account/profile", `{"label":"Alice updated"}`, []*http.Cookie{old}, 200)
	profile, err := srv.credentials.Account(ctx, a.ID)
	if err != nil || profile.Label != "Alice updated" || profile.Role != a.Role {
		t.Fatal("profile update changed account permissions")
	}
	call("PUT", "/api/account/password", `{"username":"alice","password":"changed-password-123","current_password":"wrong"}`, []*http.Cookie{old}, 400)
	changed := call("PUT", "/api/account/password", `{"username":"alice","password":"changed-password-123","current_password":"initial-password-123"}`, []*http.Cookie{old}, 200)
	current := namedCookie(t, changed, sessionCookie)
	if out := call("GET", "/api/session", "", []*http.Cookie{old}, 200); strings.Contains(out.Body.String(), `"authenticated":true`) {
		t.Fatal("old password session survived rotation")
	}
	var count int64
	if err := srv.sessions.orm.Model(&sessionRecord{}).Where(&sessionRecord{Owner: "account:" + a.ID}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("invalidated sessions still consume capacity")
	}
	call("POST", "/api/auth/password", `{"username":"alice","password":"initial-password-123"}`, nil, 401)
	setup := call("POST", "/api/account/totp/start", "{}", []*http.Cookie{current}, 200)
	var enrollment struct{ ID, Secret string }
	if err = json.Unmarshal(setup.Body.Bytes(), &enrollment); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(enrollment.Secret, srv.now())
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"id": enrollment.ID, "code": code})
	enabled := call("POST", "/api/account/totp/finish", string(payload), []*http.Cookie{current}, 200)
	namedCookie(t, enabled, sessionCookie)
	var recovery struct {
		Codes []string `json:"recovery_codes"`
	}
	if err = json.Unmarshal(enabled.Body.Bytes(), &recovery); err != nil || len(recovery.Codes) != 8 {
		t.Fatal("missing recovery codes")
	}
	encrypted, _, err := srv.credentials.TOTP(ctx, a.ID)
	if err != nil || strings.Contains(string(encrypted), enrollment.Secret) {
		t.Fatal("TOTP secret stored without encryption")
	}
	challenge := call("POST", "/api/auth/password", `{"username":"alice","password":"changed-password-123"}`, nil, 200)
	if !strings.Contains(challenge.Body.String(), `"two_factor":true`) {
		t.Fatal("2FA bypassed")
	}
	auth := namedCookie(t, challenge, "headdeck_auth")
	srv.cfg.PasswordLogin = false
	call("POST", "/api/auth/totp", `{"code":"`+code+`"}`, []*http.Cookie{auth}, 401)
	srv.cfg.PasswordLogin = true
	challenge = call("POST", "/api/auth/password", `{"username":"alice","password":"changed-password-123"}`, nil, 200)
	auth = namedCookie(t, challenge, "headdeck_auth")
	for _, cookie := range challenge.Result().Cookies() {
		if cookie.Name == sessionCookie {
			t.Fatal("session issued before 2FA")
		}
	}
	call("POST", "/api/auth/totp", `{"code":"`+code+`"}`, []*http.Cookie{auth}, 401)
	srv.now = func() time.Time { return time.Now().Add(time.Minute) }
	code, err = totp.GenerateCode(enrollment.Secret, srv.now())
	if err != nil {
		t.Fatal(err)
	}
	challenge = call("POST", "/api/auth/password", `{"username":"alice","password":"changed-password-123"}`, nil, 200)
	auth = namedCookie(t, challenge, "headdeck_auth")
	verified := call("POST", "/api/auth/totp", `{"code":"`+code+`"}`, []*http.Cookie{auth}, 200)
	current = namedCookie(t, verified, sessionCookie)
	call("POST", "/api/auth/totp", `{"code":"`+code+`"}`, []*http.Cookie{auth}, 401)
	req := httptest.NewRequest("POST", "/", nil)
	if !srv.verifyTOTP(req, a.ID, recovery.Codes[0]) || srv.verifyTOTP(req, a.ID, recovery.Codes[0]) {
		t.Fatal("recovery code was not single-use")
	}
	srv.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	restarted, err := newSessionStore(srv.sessions.db, srv.sessions.key, srv.sessions.ttl)
	if err != nil {
		t.Fatal(err)
	}
	srv.sessions = restarted
	srv.cfg.PublicURL = "https://headdeck.example"
	call("GET", "/api/account/security", "", []*http.Cookie{current}, 200)
	call("POST", "/api/account/passkeys/start", "{}", []*http.Cookie{current}, 200)
	call("PUT", "/api/account/password", `{"username":"alice","password":"another-password-123","current_password":"wrong"}`, []*http.Cookie{current}, 400)
	call("PUT", "/api/account/password", `{"username":"alice","password":"another-password-123","current_password":"changed-password-123"}`, []*http.Cookie{current}, 200)
}

func TestVerifiedProviderBindingAndIsolation(t *testing.T) {
	srv, svc, _ := testWebServer(t)
	ctx := context.Background()
	headers := map[string]string{"X-Requested-With": "headdeck"}
	a, err := svc.LoginAccount(ctx, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://github.com", Subject: "123456"}, Username: "123456", Name: "Alice"}, true)
	if err != nil || a.Username != "123456" {
		t.Fatalf("GitHub username: %+v %v", a, err)
	}
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://github.com", Subject: "123456"}}
	raw, err := srv.issueAccount(a, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: raw}
	srv.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	provider := &fixtureProvider{}
	srv.providers["corp"] = provider
	start := request(t, srv, "POST", "/api/account/providers/corp/bind", "{}", headers, []*http.Cookie{cookie})
	if start.Code != 200 {
		t.Fatalf("binding start: %s", start.Body.String())
	}
	flow := namedCookie(t, start, "headdeck_corp_flow")
	path := "/api/auth/corp/callback?code=fixture&state=" + provider.request.State
	finish := request(t, srv, "GET", path, "", nil, []*http.Cookie{flow, cookie})
	if finish.Code != 303 || !strings.Contains(finish.Header().Get("Location"), "account_security") {
		t.Fatalf("binding finish: %d %s", finish.Code, finish.Body.String())
	}
	bindings, err := srv.credentials.Bindings(ctx, a.ID)
	if err != nil || len(bindings) != 2 {
		t.Fatal("multi-binding missing")
	}
	resolved, err := svc.LoginAccount(ctx, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://login.example", Subject: "new-user"}}, false)
	if err != nil || resolved.ID != a.ID {
		t.Fatal("binding resolved to another account")
	}
	other, err := svc.LoginAccount(ctx, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://other.example", Subject: "new-user"}, Name: "Other"}, true)
	if err != nil {
		t.Fatal(err)
	}
	otherRaw, err := srv.issueAccount(other, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://other.example", Subject: "new-user"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	start = request(t, srv, "POST", "/api/account/providers/corp/bind", "{}", headers, []*http.Cookie{cookie})
	flow = namedCookie(t, start, "headdeck_corp_flow")
	path = "/api/auth/corp/callback?code=fixture&state=" + provider.request.State
	finish = request(t, srv, "GET", path, "", nil, []*http.Cookie{flow, {Name: sessionCookie, Value: otherRaw}})
	if !strings.Contains(finish.Header().Get("Location"), "invalid_flow") {
		t.Fatal("cross-session binding accepted")
	}
	for _, binding := range bindings {
		if err = srv.credentials.UnbindSelf(ctx, a.ID, binding.Identity); err != nil && len(bindings) > 1 {
			break
		}
	}
	remaining, err := srv.credentials.Bindings(ctx, a.ID)
	if err != nil || len(remaining) != 1 {
		t.Fatal("last sign-in method not protected")
	}
}
