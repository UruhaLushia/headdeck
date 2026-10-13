package web

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/github"
	"github.com/UruhaLushia/headdeck/internal/auth/oauth"
	"github.com/UruhaLushia/headdeck/internal/auth/oidc"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
	"sort"
	"time"
)

func (s *Server) provider(id string) accounts.Provider {
	s.providerMu.RLock()
	defer s.providerMu.RUnlock()
	return s.providers[id]
}
func (s *Server) providerDefinitions(ctx context.Context) ([]accounts.LoginProvider, error) {
	out, err := s.credentials.Providers(ctx)
	if err != nil {
		return nil, err
	}
	if s.cfg.OIDC.Enabled || s.cfg.OIDC.ClientID != "" || s.cfg.OIDC.Issuer != "" {
		out = append(out, accounts.LoginProvider{ID: "oidc", Name: s.cfg.OIDC.Name, Kind: "oidc", Issuer: s.cfg.OIDC.Issuer, ClientID: s.cfg.OIDC.ClientID, ClientSecret: s.cfg.OIDC.ClientSecret, Enabled: s.cfg.OIDC.Enabled, AutoRegister: s.cfg.OIDC.AutoRegister})
	}
	if s.cfg.GitHub.Enabled || s.cfg.GitHub.ClientID != "" {
		out = append(out, accounts.LoginProvider{ID: "github", Name: "GitHub", Kind: "github", ClientID: s.cfg.GitHub.ClientID, ClientSecret: s.cfg.GitHub.ClientSecret, Enabled: s.cfg.GitHub.Enabled, AutoRegister: s.cfg.GitHub.AutoRegister})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Server) loadProviders(ctx context.Context) error {
	defs, err := s.credentials.Providers(ctx)
	if err != nil {
		return err
	}
	s.providerMu.Lock()
	defer s.providerMu.Unlock()
	for id := range s.providers {
		if id != "oidc" && id != "github" {
			delete(s.providers, id)
		}
	}
	for _, p := range defs {
		if !p.Enabled {
			continue
		}
		switch p.Kind {
		case "oidc":
			s.providers[p.ID] = oidc.New(p.Issuer, p.ClientID, p.ClientSecret)
		case "github":
			s.providers[p.ID] = github.New(p.ClientID, p.ClientSecret)
		case "oauth":
			s.providers[p.ID] = oauth.New(p)
		}
	}
	return nil
}
func (s *Server) providerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/{provider}/start", func(w http.ResponseWriter, r *http.Request) { s.handleProviderStart(w, r, r.PathValue("provider")) })
	mux.HandleFunc("GET /api/auth/{provider}/callback", func(w http.ResponseWriter, r *http.Request) { s.handleProviderCallback(w, r, r.PathValue("provider")) })
	s.selfEndpoint(mux, "POST /api/account/providers/{provider}/bind", func(w http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		name := r.PathValue("provider")
		provider := s.provider(name)
		if provider == nil {
			return nil, protocol.NewError(protocol.CodeNotFound, "provider unavailable")
		}
		if !s.limiter.allow(s.clientIP(r), s.now()) {
			return nil, protocol.NewError(protocol.CodeRateLimited, "too many attempts")
		}
		flow, ok := s.flows.begin(name, s.now())
		if !ok {
			return nil, protocol.NewError(protocol.CodeRateLimited, "too many attempts")
		}
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			return nil, err
		}
		s.flows.bind(flow.state, a.ID, sessionHash(cookie.Value), a.SecurityVersion)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		location, err := provider.AuthorizationURL(ctx, accounts.LoginRequest{RedirectURI: s.redirectURI(name), State: flow.state, Nonce: flow.nonce, CodeChallenge: challenge(flow.verifier)})
		if err != nil {
			s.flows.take(flow.state, flow.secret, name, s.now())
			return nil, err
		}
		s.flowCookie(w, name, flow.secret, 300)
		return map[string]string{"url": location}, nil
	})
	read := []principal.Permission{principal.PermAccountsRead}
	write := []principal.Permission{principal.PermAccountsWrite}
	s.endpoint(mux, "GET /api/login-providers", read, func(r *http.Request) (any, error) {
		out, err := s.providerDefinitions(r.Context())
		for i := range out {
			out[i].SecretConfigured = out[i].ClientSecret != ""
			out[i].ClientSecret = ""
		}
		return out, err
	})
	s.endpoint(mux, "PUT /api/login-providers/{id}", write, withBody(func(r *http.Request, p accounts.LoginProvider) (any, error) {
		p.ID = r.PathValue("id")
		return map[string]bool{"updated": true}, s.svc.SaveLoginProvider(r.Context(), p)
	}))
	s.endpoint(mux, "DELETE /api/login-providers/{id}", write, func(r *http.Request) (any, error) {
		return map[string]bool{"updated": true}, s.svc.DeleteLoginProvider(r.Context(), r.PathValue("id"))
	})
}
