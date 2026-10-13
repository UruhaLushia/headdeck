package server

import (
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) loginProviderRoutes(mux *http.ServeMux) {
	for _, route := range []string{"GET " + protocol.PathLoginProviders, "PUT " + protocol.PathLoginProviders + "/{id}", "DELETE " + protocol.PathLoginProviders + "/{id}"} {
		mux.HandleFunc(route, s.handleLoginProviders)
	}
}
func (s *Server) handleLoginProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	permission := principal.PermAccountsRead
	if r.Method != http.MethodGet {
		permission = principal.PermAccountsWrite
	}
	if !s.guard(ctx, w, r, permission) {
		return
	}
	if r.Method == http.MethodGet {
		out, err := s.svc.LoginProviders(ctx)
		if err != nil {
			writeError(ctx, w, protocol.FromError(err))
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	var err error
	if r.Method == http.MethodDelete {
		err = s.svc.DeleteLoginProvider(ctx, r.PathValue("id"))
	} else {
		var v protocol.LoginProvider
		if problem := decodeJSON(bodyFrom(ctx), &v); problem != nil {
			writeError(ctx, w, problem)
			return
		}
		v.ID = r.PathValue("id")
		err = s.svc.SaveLoginProvider(ctx, v)
	}
	s.audit(ctx, "login-providers."+r.Method, r.PathValue("id"), result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}
