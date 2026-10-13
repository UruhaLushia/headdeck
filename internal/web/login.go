package web

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.TokenLoginEnabled() {
		s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "token login is disabled"))
		return
	}
	if !s.limiter.allow(s.clientIP(r), s.now()) {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited,
			"too many sign-in attempts; wait a minute").WithRetryable(true))
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeBadRequest, "invalid request body"))
		return
	}
	token, err := s.loadToken()
	if err != nil {
		s.logger.Named("web").Error("cannot read the web token file", "error", err.Error())
		s.writeError(w, protocol.NewError(protocol.CodeClientConfig,
			"the web listener has no usable token file"))
		return
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Token)), []byte(token)) != 1 {
		s.logger.Audit(r.Context(), log.AuditEvent{
			Actor:  "web:" + s.clientIP(r),
			Action: "web.login",
			Result: "denied:UNAUTHENTICATED",
		})
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "invalid token"))
		return
	}

	value, err := s.sessions.issue("web", principal.RoleOwner, s.now(), r.UserAgent())
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot create session"))
		return
	}

	s.setSessionCookie(w, value)
	s.logger.Audit(r.Context(), log.AuditEvent{
		Actor:  "web:" + s.clientIP(r),
		Action: "web.login",
		Result: "ok",
	})
	s.writeJSON(w, http.StatusOK, map[string]any{
		"subject": "web", "role": string(principal.RoleOwner),
		"expires_in": int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := s.sessions.revoke(cookie.Value); err != nil {
			s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot revoke session"))
			return
		}
	}
	s.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	p, current, ok := s.sessionPrincipal(r, cookie.Value)
	if !ok {
		s.writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	if p.Role == principal.RoleMember && p.HeadscaleUserID == "" && s.cfg.DefaultRole == "member" {
		if _, err := s.svc.UserSnapshot(r.Context()); err != nil {
			s.logger.Named("web").Debug("Headscale registration sync failed", "error", err.Error())
		} else {
			p, current, ok = s.sessionPrincipal(r, cookie.Value)
			if !ok {
				s.writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
				return
			}
		}
	}
	s.maybeRenewSession(w, r, current)
	permissions := permissionStrings(p)
	continueURL := ""
	loginServer := ""
	if p.Role != principal.RolePending && s.cfg.OIDCServer.Enabled {
		loginServer = strings.TrimSuffix(s.cfg.OIDCServer.RedirectURI, "/oidc/callback")
	}
	if p.Role != principal.RolePending && (p.HeadscaleUserID != "" || p.Role == principal.RoleMember && s.cfg.DefaultRole == "member") {
		continueURL = s.oidcContinuation(r)
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":       true,
		"subject":             p.ID,
		"role":                string(p.Role),
		"label":               p.Label,
		"avatar_url":          current.AvatarURL,
		"headscale_user_id":   p.HeadscaleUserID,
		"permissions":         permissions,
		"continue_url":        continueURL,
		"device_registration": p.Role == principal.RoleMember && p.HeadscaleUserID == "" && s.cfg.DefaultRole == "member",
		"login_server":        loginServer,
	})
}
