package oidcserver

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/UruhaLushia/headdeck/internal/database"
	"github.com/zitadel/oidc/v3/pkg/op"
)

type Options struct {
	Database                                                    *sql.DB
	Issuer, ClientID, ClientSecret, RedirectURI, SigningKeyFile string
	TokenTTL                                                    time.Duration
	Validate                                                    func(context.Context, Profile) error
}
type Provider struct {
	*op.Provider
	store *storage
}

func New(opts Options) (*Provider, error) {
	if opts.Database == nil || opts.Validate == nil || opts.ClientID == "" || opts.ClientSecret == "" || opts.TokenTTL <= 0 {
		return nil, errors.New("incomplete OIDC provider configuration")
	}
	key, err := loadKey(opts.SigningKeyFile)
	if err != nil {
		return nil, err
	}
	orm, err := database.Open(opts.Database)
	if err != nil {
		return nil, err
	}
	for _, model := range []any{&requestRecord{}, &tokenRecord{}} {
		if !orm.Migrator().HasTable(model) {
			if err := orm.Migrator().CreateTable(model); err != nil {
				return nil, err
			}
		}
	}
	s := &storage{db: orm, key: key, issuer: opts.Issuer, secret: opts.ClientSecret, client: client{ID: opts.ClientID, Redirect: opts.RedirectURI, Issuer: opts.Issuer, Lifetime: opts.TokenTTL}, validate: opts.Validate}
	cryptoKey := sha256.Sum256(x509.MarshalPKCS1PrivateKey(key.private))
	p, err := op.NewProvider(&op.Config{CryptoKey: cryptoKey, CodeMethodS256: true, AuthMethodPost: true, SupportedScopes: []string{"openid", "profile", "email"}, SupportedClaims: []string{"sub", "name", "preferred_username", "picture", "email", "email_verified"}}, s, op.StaticIssuer(opts.Issuer))
	if err != nil {
		return nil, err
	}
	return &Provider{Provider: p, store: s}, nil
}

func (p *Provider) Authenticate(ctx context.Context, id string, profile Profile) (string, error) {
	if err := p.store.authenticate(ctx, id, profile); err != nil {
		return "", err
	}
	return p.store.issuer + "/authorize/callback?id=" + url.QueryEscape(id), nil
}
func (p *Provider) HasRequest(ctx context.Context, id string) bool {
	_, err := p.store.AuthRequestByID(ctx, id)
	return err == nil
}
func (p *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if len(r.URL.RawQuery) > 8192 {
		http.Error(w, "Request too large", http.StatusRequestURITooLong)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	p.Provider.ServeHTTP(w, r)
}
func (p *Provider) NeedsLogin(ctx context.Context, id string, authenticatedAt time.Time) bool {
	request, err := p.store.AuthRequestByID(ctx, id)
	if err != nil {
		return true
	}
	a := request.(*authRequest)
	if slices.Contains(a.Request.Prompt, "login") && authenticatedAt.Unix() < a.RequestedAt.Unix() {
		return true
	}
	return a.Request.MaxAge != nil && uint(time.Since(authenticatedAt)/time.Second) > *a.Request.MaxAge
}
