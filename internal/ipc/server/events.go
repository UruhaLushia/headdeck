package server

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/eventstream"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermSystemRead) {
		return
	}
	p, _ := local.PrincipalFrom(r.Context())
	if err := eventstream.Serve(w, r, s.svc.Hub(), eventstream.Scopes(p), 0, func() bool { u, err := s.store.Lookup(*p.UID); return err == nil && u.Role == p.Role }); err != nil {
		writeError(r.Context(), w, err)
	}
}
