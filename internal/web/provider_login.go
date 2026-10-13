package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"golang.org/x/oauth2"
)

func (s *Server) handleLoginMethods(w http.ResponseWriter, r *http.Request) {
	name := s.cfg.OIDC.Name
	if name == "" {
		name = "Single sign-on"
	}
	defs, err := s.providerDefinitions(r.Context())
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot load login providers"))
		return
	}
	providers := []map[string]string{}
	for _, p := range defs {
		if p.Enabled {
			issuer := p.Issuer
			if p.Kind == "github" {
				issuer = "https://github.com"
			}
			if p.Kind == "oauth" {
				issuer = "urn:headdeck:oauth:" + p.ID
			}
			providers = append(providers, map[string]string{"id": p.ID, "name": p.Name, "issuer": issuer})
		}
	}
	_, passkeyErr := s.passkeyConfig()
	s.writeJSON(w, http.StatusOK, map[string]any{"oidc": s.provider("oidc") != nil, "github": s.provider("github") != nil, "name": name, "token": s.cfg.TokenLoginEnabled(), "password": s.cfg.PasswordLogin, "passkey": passkeyErr == nil, "providers": providers})
}

func (s *Server) redirectURI(name string) string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/api/auth/" + name + "/callback"
}

func (s *Server) flowCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: "headdeck_" + name + "_flow", Value: value, Path: s.basePath() + "/api/auth/" + name + "/callback", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

func (s *Server) loginFailure(w http.ResponseWriter, r *http.Request, name, reason string) {
	s.logger.Audit(r.Context(), log.AuditEvent{Actor: "web:" + s.clientIP(r), Action: "web." + name + ".login", Result: "denied:" + reason})
	http.Redirect(w, r, s.basePath()+"/?login_error="+reason, http.StatusSeeOther)
}

func (s *Server) bindingFailure(w http.ResponseWriter, r *http.Request, name, reason string) {
	s.logger.Audit(r.Context(), log.AuditEvent{Actor: actor(r), Action: "web." + name + ".bind", Result: "denied:" + reason})
	http.Redirect(w, r, s.basePath()+"/?account_security=1&binding_error="+reason, http.StatusSeeOther)
}

func (s *Server) handleProviderStart(w http.ResponseWriter, r *http.Request, name string) {
	provider := s.provider(name)
	if provider == nil {
		s.loginFailure(w, r, name, "unavailable")
		return
	}
	if !s.limiter.allow(s.clientIP(r), s.now()) {
		s.loginFailure(w, r, name, "rate_limited")
		return
	}
	flow, ok := s.flows.begin(name, s.now())
	if !ok {
		s.loginFailure(w, r, name, "rate_limited")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	location, err := provider.AuthorizationURL(ctx, accounts.LoginRequest{RedirectURI: s.redirectURI(name), State: flow.state,
		Nonce: flow.nonce, CodeChallenge: oauth2.S256ChallengeFromVerifier(flow.verifier)})
	if err != nil {
		s.flows.take(flow.state, flow.secret, name, s.now())
		s.loginFailure(w, r, name, "unavailable")
		return
	}
	s.flowCookie(w, name, flow.secret, 300)
	http.Redirect(w, r, location, http.StatusFound)
}

func (s *Server) handleProviderCallback(w http.ResponseWriter, r *http.Request, name string) {
	if len(r.URL.RawQuery) > 8192 {
		s.loginFailure(w, r, name, "invalid_flow")
		return
	}
	if s.provider(name) == nil {
		s.loginFailure(w, r, name, "unavailable")
		return
	}
	cookie, err := r.Cookie("headdeck_" + name + "_flow")
	if err != nil {
		s.loginFailure(w, r, name, "invalid_flow")
		return
	}
	flow, ok := s.flows.take(r.URL.Query().Get("state"), cookie.Value, name, s.now())
	if !ok {
		s.loginFailure(w, r, name, "invalid_flow")
		return
	}
	s.flowCookie(w, name, "", -1)
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		s.loginFailure(w, r, name, "sign_in_failed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	provider := s.provider(name)
	if provider == nil {
		s.loginFailure(w, r, name, "unavailable")
		return
	}
	identity, err := provider.Exchange(ctx, accounts.Callback{RedirectURI: s.redirectURI(name), Code: r.URL.Query().Get("code"), CodeVerifier: flow.verifier, Nonce: flow.nonce})
	if err != nil {
		if flow.account != "" {
			s.bindingFailure(w, r, name, "sign_in_failed")
			return
		}
		s.loginFailure(w, r, name, "sign_in_failed")
		return
	}
	if flow.account != "" {
		a, _, e := s.accountRequest(r)
		cookie, cookieErr := r.Cookie(sessionCookie)
		if e != nil || cookieErr != nil || a.ID != flow.account || a.SecurityVersion != flow.version || sessionHash(cookie.Value) != flow.session {
			s.bindingFailure(w, r, name, "invalid_flow")
			return
		}
		if err = s.svc.BindIdentity(ctx, a.ID, identity.Identity, false); err != nil {
			s.bindingFailure(w, r, name, "identity_conflict")
			return
		}
		if err = s.credentials.SetAvatar(ctx, a.ID, identity.AvatarURL); err != nil {
			s.bindingFailure(w, r, name, "sign_in_failed")
			return
		}
		http.Redirect(w, r, s.basePath()+"/?account_security=1", http.StatusSeeOther)
		return
	}
	a, err := s.svc.LoginAccount(ctx, identity, s.autoRegister(r.Context(), name))
	if err != nil {
		s.loginFailure(w, r, name, "account_unavailable")
		return
	}
	s.logger.Audit(r.Context(), log.AuditEvent{Actor: "account:" + a.ID, Action: "web." + name + ".login", Result: "ok"})
	s.completeLogin(w, r, a, identity, false, true)
}

func (s *Server) autoRegister(ctx context.Context, name string) bool {
	if name == "github" {
		return s.cfg.GitHub.AutoRegister
	}
	if name == "oidc" {
		return s.cfg.OIDC.AutoRegister
	}
	defs, err := s.credentials.Providers(ctx)
	if err != nil {
		return false
	}
	for _, p := range defs {
		if p.ID == name {
			return p.AutoRegister
		}
	}
	return false
}
