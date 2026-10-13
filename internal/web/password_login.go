package web

import (
	"errors"
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PasswordLogin {
		s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "password login is disabled"))
		return
	}
	if !s.limiter.allow(s.clientIP(r), s.now()) {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many sign-in attempts"))
		return
	}
	v, err := decodeRequest[passwordRequest](r)
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	a, err := s.credentials.Authenticate(r.Context(), v.Username, v.Password)
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "invalid username or password"))
		return
	}
	s.completeLogin(w, r, a, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: accounts.LocalIssuer, Subject: a.ID}}, false, false)
}
func (s *Server) challengeCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: "headdeck_auth", Value: value, Path: s.basePath() + "/api/auth", Secure: s.cfg.CookieSecure, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge})
}
func (s *Server) completeLogin(w http.ResponseWriter, r *http.Request, a *accounts.Account, identity accounts.VerifiedIdentity, multifactor, redirect bool) {
	_, _, err := s.credentials.TOTP(r.Context(), a.ID)
	if err != nil && !errors.Is(err, accounts.ErrNotFound) {
		s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot verify account security"))
		return
	}
	if err == nil && !multifactor {
		id, ok := s.authStates.put(authState{kind: "totp-login", account: a.ID, version: a.SecurityVersion, identity: identity}, s.now())
		if !ok {
			s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many sign-in attempts"))
			return
		}
		s.challengeCookie(w, id, 300)
		if redirect {
			http.Redirect(w, r, s.basePath()+"/?two_factor=1", http.StatusSeeOther)
		} else {
			s.writeJSON(w, http.StatusOK, map[string]bool{"two_factor": true})
		}
		return
	}
	value, err := s.issueAccount(a, identity, r.UserAgent())
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot create session"))
		return
	}
	s.setSessionCookie(w, value)
	s.challengeCookie(w, "", -1)
	if redirect {
		http.Redirect(w, r, s.basePath()+"/", http.StatusSeeOther)
	} else {
		s.writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
	}
}
func (s *Server) handleTOTPLogin(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow(s.clientIP(r), s.now()) {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many sign-in attempts"))
		return
	}
	cookie, err := r.Cookie("headdeck_auth")
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "sign in again"))
		return
	}
	v, err := decodeRequest[struct {
		Code string `json:"code"`
	}](r)
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	state, ok := s.authStates.take(cookie.Value, "totp-login", "", s.now())
	if !ok {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "sign in again"))
		return
	}
	a, err := s.credentials.LocalAccount(r.Context(), state.account)
	if state.identity.Issuer == accounts.LocalIssuer && !s.cfg.PasswordLogin {
		s.challengeCookie(w, "", -1)
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "password login is disabled"))
		return
	}
	if err != nil || a.SecurityVersion != state.version || !s.verifyTOTP(r, state.account, v.Code) {
		s.challengeCookie(w, "", -1)
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "invalid verification code; sign in again"))
		return
	}
	s.completeLogin(w, r, a, state.identity, true, false)
}
