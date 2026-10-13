package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"golang.org/x/oauth2"
)

const Issuer = "https://github.com"

type Provider struct {
	clientID, secret string
	client           *http.Client
}

func New(clientID, secret string) *Provider {
	return &Provider{clientID: clientID, secret: secret, client: &http.Client{
		Timeout: 10 * time.Second, Transport: transport{base: http.DefaultTransport},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (p *Provider) oauth(redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: p.clientID, ClientSecret: p.secret, RedirectURL: redirect, Scopes: []string{"read:user", "user:email"},
		Endpoint: oauth2.Endpoint{AuthURL: Issuer + "/login/oauth/authorize", TokenURL: Issuer + "/login/oauth/access_token", AuthStyle: oauth2.AuthStyleInParams}}
}

func (p *Provider) AuthorizationURL(_ context.Context, req accounts.LoginRequest) (string, error) {
	if req.State == "" || req.CodeChallenge == "" {
		return "", errors.New("GitHub login requires state and PKCE")
	}
	return p.oauth(req.RedirectURI).AuthCodeURL(req.State, oauth2.SetAuthURLParam("code_challenge", req.CodeChallenge), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}

func (p *Provider) Exchange(ctx context.Context, callback accounts.Callback) (accounts.VerifiedIdentity, error) {
	if callback.Code == "" || callback.CodeVerifier == "" {
		return accounts.VerifiedIdentity{}, errors.New("GitHub callback requires code and verifier")
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.oauth(callback.RedirectURI).Exchange(ctx, callback.Code, oauth2.VerifierOption(callback.CodeVerifier))
	if err != nil || token.AccessToken == "" {
		return accounts.VerifiedIdentity{}, errors.New("GitHub token exchange failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := p.client.Do(req)
	if err != nil {
		return accounts.VerifiedIdentity{}, errors.New("GitHub identity request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return accounts.VerifiedIdentity{}, errors.New("GitHub rejected the identity request")
	}
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&user); err != nil || user.ID <= 0 || user.Login == "" {
		return accounts.VerifiedIdentity{}, errors.New("GitHub returned an invalid identity")
	}
	if user.Name == "" {
		user.Name = user.Login
	}
	id := strconv.FormatInt(user.ID, 10)
	emails, err := p.verifiedEmails(ctx, token.AccessToken)
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	return accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: Issuer, Subject: id}, Username: user.Login, Name: user.Name, AvatarURL: "https://avatars.githubusercontent.com/u/" + id + "?s=96", VerifiedEmails: emails}, nil
}

func (p *Provider) verifiedEmails(ctx context.Context, token string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := p.client.Do(req)
	if err != nil {
		return nil, errors.New("GitHub email request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("GitHub rejected the email request")
	}
	var records []struct {
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
		Primary  bool   `json:"primary"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&records); err != nil {
		return nil, errors.New("GitHub returned invalid emails")
	}
	emails := []string{}
	for _, r := range records {
		if !r.Verified || len(r.Email) > 254 {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(r.Email))
		if !strings.Contains(email, "@") {
			continue
		}
		if len(emails) >= 10 {
			return nil, errors.New("too many verified GitHub emails")
		}
		if r.Primary {
			emails = append([]string{email}, emails...)
		} else {
			emails = append(emails, email)
		}
	}
	return emails, nil
}
