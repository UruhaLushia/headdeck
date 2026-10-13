package web

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/ipc/eventstream"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && !s.originAllowed(origin) {
		s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "invalid event stream origin"))
		return
	}
	p, _ := principalFrom(r)
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return
	}
	now := s.now()
	session, ok := s.sessions.verify(cookie.Value, now)
	if !ok {
		return
	}
	authorized := func() bool {
		if err := s.authorizeLogin(r.Context()); err != nil {
			return false
		}
		current, _, valid := s.sessionPrincipal(r, cookie.Value)
		return valid && current.ID == p.ID && current.Role == p.Role && current.HeadscaleUserID == p.HeadscaleUserID
	}
	if err := eventstream.Serve(w, r, s.svc.Hub(), eventstream.Scopes(p), session.ExpiresAt.Sub(now), authorized); err != nil {
		s.writeError(w, err)
	}
}
