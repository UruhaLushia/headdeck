package server

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+protocol.PathAccounts, s.handleAccounts)
	mux.HandleFunc("POST "+protocol.PathAccounts, s.handleAccountSave)
	mux.HandleFunc("PUT "+protocol.PathAccounts+"/{id}", s.handleAccountSave)
	mux.HandleFunc("GET "+protocol.PathAccounts+"/{id}/identities", s.handleBindings)
	mux.HandleFunc("POST "+protocol.PathAccounts+"/{id}/identities", s.handleBind)
	mux.HandleFunc("DELETE "+protocol.PathAccounts+"/{id}/identities", s.handleBind)
	mux.HandleFunc("PUT "+protocol.PathAccounts+"/{id}/password", s.handleAccountPassword)
	mux.HandleFunc("GET "+protocol.PathAccounts+"/{id}/security", s.handleAccountSecurity)
	mux.HandleFunc("POST "+protocol.PathAccounts+"/{id}/security", s.handleAccountSecurity)
}
func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermAccountsRead) {
		return
	}
	out, err := s.svc.Accounts(ctx)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, protocol.AccountList{Accounts: out})
}
func (s *Server) handleAccountSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermAccountsWrite) {
		return
	}
	var req protocol.AccountCredentials
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	out, err := s.svc.SaveAccountCredentials(ctx, r.PathValue("id"), req.Account, req.Password)
	id := r.PathValue("id")
	if err == nil {
		id = out.ID
	}
	s.audit(ctx, "accounts.save", id, result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) handleBindings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermAccountsRead) {
		return
	}
	out, err := s.svc.IdentityBindings(ctx, r.PathValue("id"))
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, protocol.BindingList{Bindings: out})
}
func (s *Server) handleBind(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermAccountsWrite) {
		return
	}
	var req protocol.Identity
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	err := s.svc.BindIdentity(ctx, r.PathValue("id"), req, r.Method == http.MethodDelete)
	action := "accounts.bind"
	if r.Method == http.MethodDelete {
		action = "accounts.unbind"
	}
	s.audit(ctx, action, r.PathValue("id"), result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
