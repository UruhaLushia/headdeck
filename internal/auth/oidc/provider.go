package oidc

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Provider struct {
	issuer, clientID, secret string
	client                   *http.Client
	mu                       sync.Mutex
	provider                 *coreoidc.Provider
	loading                  chan struct{}
	lastError                error
	retryAt                  time.Time
}

func New(issuer, clientID, secret string) *Provider {
	return &Provider{issuer: issuer, clientID: clientID, secret: secret,
		client: &http.Client{Timeout: 10 * time.Second, Transport: limitedTransport{base: http.DefaultTransport}}}
}

func (p *Provider) discover(ctx context.Context) (*coreoidc.Provider, error) {
	p.mu.Lock()
	if p.provider != nil {
		provider := p.provider
		p.mu.Unlock()
		return provider, nil
	}
	if time.Now().Before(p.retryAt) {
		err := p.lastError
		p.mu.Unlock()
		return nil, err
	}
	if p.loading != nil {
		ready := p.loading
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ready:
			return p.discover(ctx)
		}
	}
	p.loading = make(chan struct{})
	p.mu.Unlock()
	provider, err := coreoidc.NewProvider(coreoidc.ClientContext(ctx, p.client), p.issuer)
	if err == nil {
		for _, endpoint := range []string{provider.Endpoint().AuthURL, provider.Endpoint().TokenURL} {
			u, parseErr := url.Parse(endpoint)
			if parseErr != nil {
				err = parseErr
				break
			}
			if err = safeURL(u); err != nil {
				break
			}
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	close(p.loading)
	p.loading = nil
	if err != nil {
		p.lastError = err
		p.retryAt = time.Now().Add(30 * time.Second)
		return nil, err
	}
	p.provider = provider
	return provider, nil
}

func (p *Provider) oauth(provider *coreoidc.Provider, redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: p.clientID, ClientSecret: p.secret,
		Endpoint: provider.Endpoint(), RedirectURL: redirect,
		Scopes: []string{coreoidc.ScopeOpenID, "profile", "email"}}
}

func (p *Provider) AuthorizationURL(ctx context.Context, req accounts.LoginRequest) (string, error) {
	provider, err := p.discover(ctx)
	if err != nil {
		return "", err
	}
	return p.oauth(provider, req.RedirectURI).AuthCodeURL(req.State, coreoidc.Nonce(req.Nonce),
		oauth2.SetAuthURLParam("code_challenge", req.CodeChallenge), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}

func (p *Provider) Exchange(ctx context.Context, callback accounts.Callback) (accounts.VerifiedIdentity, error) {
	provider, err := p.discover(ctx)
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	token, err := p.oauth(provider, callback.RedirectURI).Exchange(coreoidc.ClientContext(ctx, p.client), callback.Code,
		oauth2.VerifierOption(callback.CodeVerifier))
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return accounts.VerifiedIdentity{}, errors.New("provider returned no ID token")
	}
	verified, err := provider.Verifier(&coreoidc.Config{ClientID: p.clientID}).Verify(coreoidc.ClientContext(ctx, p.client), raw)
	if err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	if callback.Nonce == "" || subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(callback.Nonce)) != 1 || verified.Subject == "" {
		return accounts.VerifiedIdentity{}, errors.New("invalid identity or nonce")
	}
	var claims struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Picture           string `json:"picture"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
	}
	if err := verified.Claims(&claims); err != nil {
		return accounts.VerifiedIdentity{}, err
	}
	var emails []string
	if claims.EmailVerified {
		email := strings.ToLower(strings.TrimSpace(claims.Email))
		if address, err := mail.ParseAddress(email); err == nil && address.Address == email && len(email) <= 254 {
			emails = []string{email}
		}
	}
	return accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: verified.Issuer, Subject: verified.Subject}, Username: claims.PreferredUsername, Name: claims.Name, AvatarURL: claims.Picture, VerifiedEmails: emails}, nil
}
