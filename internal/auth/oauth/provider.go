package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"time"
)

type Provider struct {
	config accounts.LoginProvider
	client *http.Client
}

func New(config accounts.LoginProvider) *Provider {
	return &Provider{config: config, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (p *Provider) oauth(redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: p.config.ClientID, ClientSecret: p.config.ClientSecret, RedirectURL: redirect, Scopes: p.config.Scopes, Endpoint: oauth2.Endpoint{AuthURL: p.config.AuthURL, TokenURL: p.config.TokenURL}}
}
func (p *Provider) AuthorizationURL(_ context.Context, req accounts.LoginRequest) (string, error) {
	return p.oauth(req.RedirectURI).AuthCodeURL(req.State, oauth2.SetAuthURLParam("code_challenge", req.CodeChallenge), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}
func (p *Provider) Exchange(ctx context.Context, callback accounts.Callback) (accounts.VerifiedIdentity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.oauth(callback.RedirectURI).Exchange(ctx, callback.Code, oauth2.VerifierOption(callback.CodeVerifier))
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.UserinfoURL, nil)
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	response, err := p.client.Do(req)
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return accounts.VerifiedIdentity{}, errors.New("identity request failed")
	}
	var claims map[string]any
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.UseNumber()
	if err = decoder.Decode(&claims); err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	value := func(claim, fallback string) string {
		if claim == "" {
			claim = fallback
		}
		switch v := claims[claim].(type) {
		case string:
			return v
		case json.Number:
			return string(v)
		}
		return ""
	}
	subject := value(p.config.SubjectClaim, "id")
	if subject == "" {
		return accounts.VerifiedIdentity{}, errors.New("provider returned no stable identity")
	}
	username := value("preferred_username", "")
	if username == "" {
		username = value("login", "")
	}
	return accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "urn:headdeck:oauth:" + p.config.ID, Subject: subject}, Username: username, Name: value(p.config.NameClaim, "name"), AvatarURL: value(p.config.AvatarClaim, "avatar_url")}, nil
}
