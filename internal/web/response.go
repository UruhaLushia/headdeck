package web

import (
	"encoding/json"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // attributes set explicitly
		Name: sessionCookie, Value: "", Path: s.basePath() + "/", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		s.logger.Named("web").Debug("response write failed", "error", err.Error())
	}
}

func (s *Server) writeError(w http.ResponseWriter, err *protocol.Error) {
	if err == nil {
		return
	}
	s.writeJSON(w, protocol.StatusForCode(err.Code), protocol.ErrorResponse{Error: err})
}

func permissionStrings(p *principal.Principal) []string {
	perms := p.PermissionList()
	out := make([]string, 0, len(perms))
	for _, perm := range perms {
		out = append(out, string(perm))
	}
	return out
}
