package server

import (
	"net/http"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) handleOIDCTakeover(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermAccountsWrite) || !s.guard(r.Context(), w, r, principal.PermServiceManage) {
		return
	}
	p, _ := local.PrincipalFrom(r.Context())
	recover := strings.HasSuffix(r.URL.Path, "/recover")
	op, err := s.svc.ApplyOIDCTakeover(r.Context(), p.String(), recover)
	s.audit(r.Context(), "headscale.login."+strings.TrimPrefix(r.URL.Path, protocol.PathHeadscaleLogin+"/"), "Headscale login", "submitted", err)
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusAccepted, op)
}
