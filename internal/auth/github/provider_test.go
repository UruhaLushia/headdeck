package github

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGitHubVerifiedIdentity(t *testing.T) {
	p := New("client", "private-secret")
	location, err := p.AuthorizationURL(context.Background(), accounts.LoginRequest{
		RedirectURI: "https://headdeck.example/headdeck/api/auth/github/callback", State: "state", CodeChallenge: "challenge",
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(location)
	if err != nil || u.Host != "github.com" || u.Query().Get("code_challenge_method") != "S256" || u.Query().Get("state") != "state" || u.Query().Get("scope") != "read:user user:email" {
		t.Fatal("invalid authorization request")
	}
	user := `{"id":42,"login":"xishang0128","name":"汐殇"}`
	status := http.StatusOK
	p.client.Transport = transport{base: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		body := user
		responseStatus := status
		if r.URL.Host == "github.com" {
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("code_verifier") != "verifier" || r.Form.Get("client_secret") != "private-secret" || r.Header.Get("Accept") != "application/json" {
				t.Fatal("invalid token request")
			}
			body, responseStatus = `{"access_token":"transient-token","token_type":"bearer"}`, http.StatusOK
		} else if r.URL.Host != "api.github.com" || (r.URL.Path != "/user" && r.URL.Path != "/user/emails") || r.Header.Get("Authorization") != "Bearer transient-token" {
			t.Fatal("identity was not checked with GitHub")
		}
		if r.URL.Path == "/user/emails" {
			body = `[{"email":"owner@example.com","verified":true,"primary":true},{"email":"unverified@example.com","verified":false}]`
		}
		return &http.Response{StatusCode: responseStatus, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	callback := accounts.Callback{Code: "code", CodeVerifier: "verifier", RedirectURI: "https://headdeck.example/headdeck/api/auth/github/callback"}
	identity, err := p.Exchange(context.Background(), callback)
	if err != nil || identity.Issuer != Issuer || identity.Subject != "42" || identity.Username != "xishang0128" || identity.Name != "汐殇" || len(identity.VerifiedEmails) != 1 || identity.VerifiedEmails[0] != "owner@example.com" {
		t.Fatalf("identity = %+v, error = %v", identity, err)
	}
	user = `{"id":42,"login":"renamed"}`
	renamed, err := p.Exchange(context.Background(), callback)
	if err != nil || renamed.Identity != identity.Identity || renamed.Username != "renamed" || renamed.Name != "renamed" {
		t.Fatal("username changes altered the binding")
	}
	for _, invalid := range []string{`{"id":0}`, `{"id":-1}`, `{"login":"alice"}`, `{"id":42}`} {
		user = invalid
		if _, err := p.Exchange(context.Background(), callback); err == nil {
			t.Fatal("unverified identity accepted")
		}
	}
	status = http.StatusUnauthorized
	if _, err := p.Exchange(context.Background(), callback); err == nil || strings.Contains(err.Error(), "transient-token") {
		t.Fatal("rejected identity accepted or token exposed")
	}
}
