package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
)

type accountSession struct {
	ID              string            `json:"id"`
	Identity        accounts.Identity `json:"identity"`
	Version         uint64            `json:"version"`
	AvatarURL       string            `json:"avatar_url,omitempty"`
	VerifiedEmails  []string          `json:"verified_emails,omitempty"`
	AuthenticatedAt time.Time         `json:"-"`
}

func safeAvatar(value string) string {
	u, err := url.Parse(value)
	if err != nil || len(value) > 512 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return ""
	}
	return value
}

func (s *Server) issueAccount(a *accounts.Account, identity accounts.VerifiedIdentity, agent string) (string, error) {
	payload, err := json.Marshal(accountSession{ID: a.ID, Identity: identity.Identity, Version: a.SecurityVersion, AvatarURL: safeAvatar(identity.AvatarURL), VerifiedEmails: identity.VerifiedEmails})
	if err != nil {
		return "", err
	}
	return s.sessions.issue("account:"+base64.RawURLEncoding.EncodeToString(payload), "", s.now(), agent)
}

func (s *Server) sessionPrincipal(r *http.Request, value string) (*principal.Principal, session, bool) {
	current, ok := s.sessions.verify(value, s.now())
	if !ok {
		return nil, session{}, false
	}
	if encoded, account := strings.CutPrefix(current.Subject, "account:"); account {
		payload, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, session{}, false
		}
		var identity accountSession
		if err := json.Unmarshal(payload, &identity); err != nil {
			return nil, session{}, false
		}
		a, err := s.svc.LoginAccount(r.Context(), accounts.VerifiedIdentity{Identity: identity.Identity}, false)
		if err != nil || a.ID != identity.ID || a.SecurityVersion != identity.Version {
			return nil, session{}, false
		}
		p := principal.New(principal.KindWebSession, a.ID, a.Role).WithLabel(a.Label)
		p.HeadscaleUserID = a.HeadscaleUserID
		current.AvatarURL = safeAvatar(a.AvatarURL)
		if current.AvatarURL == "" {
			current.AvatarURL = safeAvatar(identity.AvatarURL)
		}
		return p, current, true
	}
	if !s.cfg.TokenLoginEnabled() {
		return nil, session{}, false
	}
	return principal.New(principal.KindWebSession, current.Subject, current.Role), current, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: value, Path: s.basePath() + "/", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: int(s.cfg.SessionTTL / time.Second), Expires: s.now().Add(s.cfg.SessionTTL)})
}

func (s *Server) maybeRenewSession(w http.ResponseWriter, r *http.Request, current session) {
	if r.Method != http.MethodGet || r.URL.Path == "/api/events" || r.URL.Path == "/api/status/stream" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && !s.originAllowed(origin) {
		return
	}
	now := s.now()
	if current.ExpiresAt.Sub(now) > s.sessions.ttl/2 {
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return
	}
	renewed, err := s.sessions.renew(cookie.Value, current, now)
	if err != nil {
		s.logger.Named("web").Warn("cannot renew session", "error", err.Error())
		return
	}
	if renewed {
		s.setSessionCookie(w, cookie.Value)
	}
}
