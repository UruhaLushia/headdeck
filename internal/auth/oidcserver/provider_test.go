package oidcserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/UruhaLushia/headdeck/internal/config"
	verifier "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func TestHeadscaleAuthorization(t *testing.T) {
	ctx := context.Background()
	db, err := config.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var provider *Provider
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/admin/oidc", provider).ServeHTTP(w, r)
	}))
	defer server.Close()
	var denied atomic.Bool
	opts := Options{Database: db.Database(), Issuer: server.URL + "/admin/oidc", ClientID: "headscale", ClientSecret: strings.Repeat("s", 32), RedirectURI: "https://headscale.example/oidc/callback", SigningKeyFile: filepath.Join(t.TempDir(), "key.pem"), TokenTTL: time.Hour, Validate: func(_ context.Context, p Profile) error {
		if denied.Load() || p.AccountID != "account" {
			return errors.New("denied")
		}
		return nil
	}}
	provider, err = New(opts)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	ctx = verifier.ClientContext(ctx, client)
	rp, err := verifier.NewProvider(ctx, opts.Issuer)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Authorization string `json:"authorization_endpoint"`
		Token         string `json:"token_endpoint"`
		Userinfo      string `json:"userinfo_endpoint"`
		Keys          string `json:"jwks_uri"`
	}
	if err := rp.Claims(&metadata); err != nil {
		t.Fatal(err)
	}
	profile := Profile{AccountID: "account", HeadscaleUserID: "10", Username: "xishang0128", Name: "汐殇", Email: "owner@example.com", EmailVerified: true, Version: 1, AuthenticatedAt: time.Now()}
	get := func(location string) *http.Response {
		t.Helper()
		response, err := client.Get(location)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	code := func(challenge string) string {
		t.Helper()
		query := url.Values{"client_id": {opts.ClientID}, "redirect_uri": {opts.RedirectURI}, "response_type": {"code"}, "scope": {"openid profile email"}, "state": {"browser-state"}, "nonce": {"nonce"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
		response := get(metadata.Authorization + "?" + query.Encode())
		response.Body.Close()
		login, err := url.Parse(response.Header.Get("Location"))
		if err != nil || login.Query().Get("id") == "" {
			t.Fatalf("authorize: %d %s", response.StatusCode, response.Header.Get("Location"))
		}
		id := login.Query().Get("id")
		target, err := provider.Authenticate(ctx, id, profile)
		if err != nil {
			t.Fatal(err)
		}
		// Restarting Headdeck must retain the in-flight request and signing key.
		provider, err = New(opts)
		if err != nil {
			t.Fatal(err)
		}
		response = get(target)
		response.Body.Close()
		callback, err := url.Parse(response.Header.Get("Location"))
		if err != nil || callback.Query().Get("code") == "" || callback.Query().Get("state") != "browser-state" {
			t.Fatalf("callback: %d %s", response.StatusCode, response.Header.Get("Location"))
		}
		return callback.Query().Get("code")
	}
	exchange := func(code, pkce, secret string) (int, []byte) {
		t.Helper()
		values := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {opts.RedirectURI}, "code_verifier": {pkce}}
		req, err := http.NewRequestWithContext(ctx, "POST", metadata.Token, strings.NewReader(values.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(opts.ClientID, secret)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body
	}
	pkce := oauth2.GenerateVerifier()
	authorizationCode := code(oauth2.S256ChallengeFromVerifier(pkce))
	status, body := exchange(authorizationCode, pkce, opts.ClientSecret)
	if status != 200 {
		t.Fatalf("exchange: %d %s", status, body)
	}
	var tokens struct {
		ID     string `json:"id_token"`
		Access string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil {
		t.Fatal(err)
	}
	jwt, err := rp.Verifier(&verifier.Config{ClientID: opts.ClientID}).Verify(ctx, tokens.ID)
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Username string `json:"preferred_username"`
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err := jwt.Claims(&claims); err != nil {
		t.Fatal(err)
	}
	if jwt.Subject != "account" || jwt.Nonce != "nonce" || claims.Username != profile.Username || claims.Email != profile.Email || !claims.Verified {
		t.Fatalf("wrong identity: %+v %+v", jwt, claims)
	}
	if _, err := rp.Verifier(&verifier.Config{ClientID: "other-client"}).Verify(ctx, tokens.ID); err == nil {
		t.Fatal("wrong audience accepted")
	}
	if status, _ := exchange(authorizationCode, pkce, opts.ClientSecret); status == 200 {
		t.Fatal("authorization code replayed")
	}
	badCode := code(oauth2.S256ChallengeFromVerifier(pkce))
	if status, _ := exchange(badCode, "wrong-verifier", opts.ClientSecret); status == 200 {
		t.Fatal("invalid PKCE accepted")
	}
	if status, _ := exchange(code(oauth2.S256ChallengeFromVerifier(pkce)), pkce, "wrong-secret"); status == 200 {
		t.Fatal("invalid client secret accepted")
	}
	info, err := rp.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokens.Access}))
	if err != nil || info.Subject != "account" {
		t.Fatalf("userinfo: %+v %v", info, err)
	}
	response := get(metadata.Keys)
	keys, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	var jwks struct{ Keys []map[string]any }
	if err := json.Unmarshal(keys, &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 1 || jwks.Keys[0]["d"] != nil {
		t.Fatal("private signing key exposed")
	}
	denied.Store(true)
	if _, err := rp.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokens.Access})); err == nil {
		t.Fatal("revoked identity retained access")
	}
	denied.Store(false)
	expired := code(oauth2.S256ChallengeFromVerifier(pkce))
	if err := provider.store.db.Model(&requestRecord{}).Where("code_hash = ?", digest(expired)).Update("expires", time.Now().Add(-time.Second).Unix()).Error; err != nil {
		t.Fatal(err)
	}
	if status, _ := exchange(expired, pkce, opts.ClientSecret); status == 200 {
		t.Fatal("expired code accepted")
	}
}
