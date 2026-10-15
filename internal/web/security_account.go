package web

import (
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"net/http"
)

type passwordRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	CurrentPassword string `json:"current_password"`
}

func (s *Server) accountRequest(r *http.Request) (*accounts.Account, session, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, session{}, protocol.NewError(protocol.CodeUnauthenticated, "sign in first")
	}
	p, current, ok := s.sessionPrincipal(r, cookie.Value)
	if !ok {
		return nil, session{}, protocol.NewError(protocol.CodeUnauthenticated, "session expired")
	}
	a, err := s.credentials.LocalAccount(r.Context(), p.ID)
	if err != nil {
		return nil, session{}, protocol.NewError(protocol.CodePermissionDenied, "account login required")
	}
	return a, current, nil
}
func (s *Server) renewAccount(w http.ResponseWriter, r *http.Request, id string) error {
	a, err := s.credentials.LocalAccount(r.Context(), id)
	if err != nil {
		return err
	}
	value, err := s.issueAccount(a, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: accounts.LocalIssuer, Subject: id}}, r.UserAgent())
	if err != nil {
		return err
	}
	s.setSessionCookie(w, value)
	return nil
}
func (s *Server) selfEndpoint(mux *http.ServeMux, route string, call func(http.ResponseWriter, *http.Request, *accounts.Account) (any, error)) {
	mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		a, current, err := s.accountRequest(r)
		if err != nil {
			s.writeError(w, protocol.FromError(err))
			return
		}
		s.maybeRenewSession(w, r, current)
		out, err := call(w, r, a)
		if r.Method != http.MethodGet {
			result := "ok"
			if err != nil {
				result = "error"
			}
			s.logger.Audit(r.Context(), log.AuditEvent{Actor: "account:" + a.ID, Action: route, Result: result})
		}
		if err != nil {
			s.writeError(w, protocol.FromError(err))
			return
		}
		s.writeJSON(w, http.StatusOK, out)
	})
}
func (s *Server) securityRoutes(mux *http.ServeMux) {
	s.selfEndpoint(mux, "PUT /api/account/profile", func(_ http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		v, err := decodeRequest[struct {
			Label string `json:"label"`
		}](r)
		if err != nil {
			return nil, err
		}
		if err := s.credentials.SetLabel(r.Context(), a.ID, v.Label); err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "%v", err)
		}
		return map[string]bool{"updated": true}, nil
	})
	mux.HandleFunc("POST /api/auth/password", s.handlePasswordLogin)
	mux.HandleFunc("POST /api/auth/totp", s.handleTOTPLogin)
	s.selfEndpoint(mux, "GET /api/account/security", func(_ http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		bindings, err := s.credentials.Bindings(r.Context(), a.ID)
		if err != nil {
			return nil, err
		}
		keys, err := s.credentials.Passkeys(r.Context(), a.ID)
		if err != nil {
			return nil, err
		}
		password, err := s.credentials.HasPassword(r.Context(), a.ID)
		if err != nil {
			return nil, err
		}
		secret, _, err := s.credentials.TOTP(r.Context(), a.ID)
		if err != nil && err != accounts.ErrNotFound {
			return nil, err
		}
		return map[string]any{"account": a, "password": password, "identities": bindings, "passkeys": keys, "totp": len(secret) > 0}, nil
	})
	s.selfEndpoint(mux, "PUT /api/account/password", func(w http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		v, err := decodeRequest[passwordRequest](r)
		if err != nil {
			return nil, err
		}
		hasPassword, err := s.credentials.HasPassword(r.Context(), a.ID)
		if err != nil {
			return nil, err
		}
		if hasPassword {
			if !s.limiter.allow(s.clientIP(r), s.now()) {
				return nil, protocol.NewError(protocol.CodePermissionDenied, "too many sign-in attempts")
			}
			verified, err := s.credentials.Authenticate(r.Context(), a.Username, v.CurrentPassword)
			if err != nil || verified.ID != a.ID {
				return nil, protocol.NewError(protocol.CodeBadRequest, "invalid username or password")
			}
		}
		if err = s.svc.SetAccountPassword(r.Context(), a.ID, v.Username, v.Password); err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "%v", err)
		}
		return map[string]bool{"updated": true}, s.renewAccount(w, r, a.ID)
	})
	s.endpoint(mux, "PUT /api/accounts/{id}/password", nil, withBody(func(r *http.Request, v passwordRequest) (any, error) {
		p, ok := principalFrom(r)
		if !ok || !p.Has("accounts.write") {
			return nil, protocol.NewError(protocol.CodePermissionDenied, "administrator required")
		}
		if _, err := s.credentials.Account(r.Context(), r.PathValue("id")); err != nil {
			return nil, err
		}
		return map[string]bool{"updated": true}, s.svc.SetAccountPassword(r.Context(), r.PathValue("id"), v.Username, v.Password)
	}))
	s.selfEndpoint(mux, "DELETE /api/account/identities", func(w http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		v, err := decodeRequest[accounts.Identity](r)
		if err != nil {
			return nil, err
		}
		if err = s.credentials.UnbindSelf(r.Context(), a.ID, v); err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "%v", err)
		}
		return map[string]bool{"updated": true}, s.renewAccount(w, r, a.ID)
	})
	s.totpRoutes(mux)
	s.passkeyRoutes(mux)
}
