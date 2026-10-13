package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) originAllowed(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if s.cfg.PublicURL != "" {
		public, err := url.Parse(s.cfg.PublicURL)
		return err == nil && strings.TrimSuffix(origin, "/") == public.Scheme+"://"+public.Host
	}
	return u.Host == s.cfg.Listen
}

// All protected Web routes require this middleware.
func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated,
				"no session; sign in first"))
			return
		}
		p, current, ok := s.sessionPrincipal(r, cookie.Value)
		if !ok {
			s.clearCookie(w)
			s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated,
				"session expired or invalid"))
			return
		}
		if p.Role == principal.RolePending || (p.Role == principal.RoleMember && p.HeadscaleUserID == "") {
			s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "waiting for administrator approval"))
			return
		}
		s.maybeRenewSession(w, r, current)
		next(w, r.WithContext(withPrincipal(r, p)))
	}
}

func (s *Server) requirePermission(perm principal.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r)
		if !ok || !p.Has(perm) {
			s.writeError(w, protocol.NewError(protocol.CodePermissionDenied,
				"this session may not access that resource"))
			return
		}
		next(w, r)
	}
}
