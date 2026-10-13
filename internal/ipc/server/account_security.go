package server

import (
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) handleAccountPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermAccountsWrite) {
		return
	}
	var v struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(bodyFrom(ctx), &v); err != nil {
		writeError(ctx, w, err)
		return
	}
	err := s.svc.SetAccountPassword(ctx, r.PathValue("id"), v.Username, v.Password)
	s.audit(ctx, "accounts.password", r.PathValue("id"), result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}
func (s *Server) handleAccountSecurity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	perm := principal.PermAccountsRead
	if r.Method == http.MethodPost {
		perm = principal.PermAccountsWrite
	}
	if !s.guard(ctx, w, r, perm) {
		return
	}
	if r.Method == http.MethodGet {
		out, err := s.svc.AccountSecurity(ctx, r.PathValue("id"))
		if err != nil {
			writeError(ctx, w, protocol.FromError(err))
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	var v protocol.AccountSecurityAction
	if err := decodeJSON(bodyFrom(ctx), &v); err != nil {
		writeError(ctx, w, err)
		return
	}
	err := s.svc.AccountSecurityAction(ctx, r.PathValue("id"), v)
	s.audit(ctx, "accounts."+v.Action, r.PathValue("id"), result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}
