package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/oidcserver"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
)

const oidcFlowCookie = "headdeck_oidc"

func (s *Server) oidcRoutes(mux *http.ServeMux) error {
	o := s.cfg.OIDCServer
	if !o.Enabled {
		return nil
	}
	p, err := oidcserver.New(oidcserver.Options{Database: s.settings.Database(), Issuer: s.cfg.OIDCIssuer(), ClientID: o.ClientID, ClientSecret: o.ClientSecret, RedirectURI: o.RedirectURI, SigningKeyFile: o.SigningKeyFile, TokenTTL: o.TokenTTL, Validate: s.validateOIDCProfile})
	if err != nil {
		return err
	}
	s.oidcServer = p
	mux.HandleFunc("GET /oidc/login", s.handleOIDCLogin)
	for _, method := range []string{"GET", "POST"} {
		mux.Handle(method+" /oidc/", http.StripPrefix("/oidc", p))
	}
	return nil
}
func (s *Server) oidcCookie(w http.ResponseWriter, id string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: oidcFlowCookie, Value: id, Path: s.basePath() + "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}
func (s *Server) oidcContinuation(r *http.Request) string {
	if s.oidcServer == nil {
		return ""
	}
	cookie, err := r.Cookie(oidcFlowCookie)
	if err != nil || len(cookie.Value) > 256 || !s.oidcServer.HasRequest(r.Context(), cookie.Value) {
		return ""
	}
	return s.basePath() + "/oidc/login?id=" + url.QueryEscape(cookie.Value)
}
func (s *Server) oidcSession(r *http.Request) (accountSession, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return accountSession{}, err
	}
	current, ok := s.sessions.verify(cookie.Value, s.now())
	if !ok {
		return accountSession{}, errors.New("session expired")
	}
	encoded, ok := strings.CutPrefix(current.Subject, "account:")
	if !ok {
		return accountSession{}, errors.New("account session required")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return accountSession{}, err
	}
	var identity accountSession
	if err := json.Unmarshal(payload, &identity); err != nil {
		return accountSession{}, err
	}
	identity.AuthenticatedAt = current.CreatedAt
	return identity, nil
}
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeLogin(r.Context()); err != nil {
		http.Error(w, "Headscale login takeover is not ready", http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" || len(id) > 256 || !s.oidcServer.HasRequest(r.Context(), id) {
		http.Error(w, "Login expired", http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.oidcCookie(w, id, 300)
	a, _, err := s.accountRequest(r)
	identity, identityErr := s.oidcSession(r)
	if err != nil || identityErr != nil || s.oidcServer.NeedsLogin(r.Context(), id, identity.AuthenticatedAt) {
		http.Redirect(w, r, s.basePath()+"/?sign_in=1", http.StatusSeeOther)
		return
	}
	if a.Role == policy.RolePending || a.HeadscaleUserID == "" && s.cfg.DefaultRole != "member" {
		http.Redirect(w, r, s.basePath()+"/", http.StatusSeeOther)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	profile, err := s.oidcProfile(ctx, a, identity)
	if err != nil {
		http.Error(w, "Headscale account binding is not ready", http.StatusForbidden)
		return
	}
	target, err := s.oidcServer.Authenticate(ctx, id, profile)
	if err != nil {
		http.Error(w, "Login rejected", http.StatusForbidden)
		return
	}
	s.oidcCookie(w, "", -1)
	http.Redirect(w, r, target, http.StatusSeeOther)
}
func (s *Server) oidcProfile(ctx context.Context, a *accounts.Account, i accountSession) (oidcserver.Profile, error) {
	u, err := s.svc.OIDCUser(ctx, a, s.cfg.OIDCIssuer())
	if err != nil {
		return oidcserver.Profile{}, err
	}
	email := strings.ToLower(strings.TrimSpace(u.Email))
	if email == "" && len(i.VerifiedEmails) > 0 {
		email = i.VerifiedEmails[0]
	}
	return oidcserver.Profile{AccountID: a.ID, HeadscaleUserID: u.ID, Identity: i.Identity, Name: a.Label, Username: u.Name, Email: email, EmailVerified: email != "" && slices.Contains(i.VerifiedEmails, email), Picture: i.AvatarURL, Version: a.SecurityVersion, AuthenticatedAt: i.AuthenticatedAt}, nil
}
func (s *Server) validateOIDCProfile(ctx context.Context, p oidcserver.Profile) error {
	if err := s.authorizeLogin(ctx); err != nil {
		return err
	}
	a, err := s.credentials.Resolve(ctx, p.Identity)
	if err != nil {
		return err
	}
	if a.ID != p.AccountID || a.Role == policy.RolePending || a.Disabled || a.SecurityVersion != p.Version {
		return errors.New("headscale login is not permitted")
	}
	if p.HeadscaleUserID == "" {
		if s.cfg.DefaultRole != "member" || a.Role != policy.RoleMember || a.Username == "" || a.Username != p.Username {
			return errors.New("headscale registration is not permitted")
		}
	} else if a.HeadscaleUserID != p.HeadscaleUserID {
		return errors.New("headscale binding changed")
	}
	users, err := s.svc.Headscale().ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if p.HeadscaleUserID == "" && u.ProviderID == s.cfg.OIDCIssuer()+"/"+p.AccountID {
			if u.Name == p.Username && u.Provider == "oidc" && (a.HeadscaleUserID == "" || a.HeadscaleUserID == u.ID) {
				return nil
			}
			return errors.New("headscale binding changed")
		}
		if u.ID == p.HeadscaleUserID && u.Name == p.Username && u.ProviderID == s.cfg.OIDCIssuer()+"/"+p.AccountID && (u.Email == "" || strings.EqualFold(u.Email, p.Email)) {
			return nil
		}
	}
	if p.HeadscaleUserID == "" && a.HeadscaleUserID == "" {
		for _, u := range users {
			if strings.EqualFold(u.Name, p.Username) {
				return errors.New("headscale username is already reserved")
			}
		}
		return nil
	}
	return errors.New("headscale binding changed")
}
