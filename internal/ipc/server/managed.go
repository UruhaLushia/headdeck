package server

import (
	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) managedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+protocol.PathHeadscaleLogin+"/apply", s.handleOIDCTakeover)
	mux.HandleFunc("POST "+protocol.PathHeadscaleLogin+"/recover", s.handleOIDCTakeover)
	for _, method := range []string{"GET", "PATCH", "DELETE", "POST"} {
		path := protocol.PathManagedConfig
		if method == "POST" {
			path += "/apply"
		}
		mux.HandleFunc(method+" "+path, s.handleManagedConfig)
	}
	for _, method := range []string{"GET", "PUT", "DELETE", "POST"} {
		path := protocol.PathPolicy
		if method == "POST" {
			path += "/apply"
		}
		mux.HandleFunc(method+" "+path, s.handlePolicy)
	}
	mux.HandleFunc("POST "+protocol.PathPolicy+"/check", s.handlePolicyCheck)
}

func (s *Server) handleManagedConfig(w http.ResponseWriter, r *http.Request) {
	permission := principal.PermSettingsRead
	if r.Method != http.MethodGet {
		permission = principal.PermSettingsWrite
	}
	if !s.guard(r.Context(), w, r, permission) {
		return
	}
	var out any
	var err error
	status := http.StatusOK
	switch r.Method {
	case http.MethodGet:
		out, err = s.svc.ManagedConfig(r.Context())
	case http.MethodPatch:
		var req protocol.ManagedConfigUpdate
		if perr := decodeJSON(bodyFrom(r.Context()), &req); perr != nil {
			writeError(r.Context(), w, perr)
			return
		}
		out, err = s.svc.SaveManagedConfig(r.Context(), req)
	case http.MethodDelete:
		err = s.svc.DiscardManagedConfig(r.Context())
		out = map[string]bool{"discarded": true}
	case http.MethodPost:
		if !s.guard(r.Context(), w, r, principal.PermServiceManage) {
			return
		}
		var req protocol.DNSApply
		if perr := decodeJSON(bodyFrom(r.Context()), &req); perr != nil {
			writeError(r.Context(), w, perr)
			return
		}
		p, _ := local.PrincipalFrom(r.Context())
		out, err = s.svc.ApplyManagedConfig(r.Context(), req.Revision, p.String())
		status = http.StatusAccepted
	}
	if r.Method != http.MethodGet {
		s.audit(r.Context(), "headscale.config."+r.Method, "Headscale configuration", "completed", err)
	}
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, status, out)
}

func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	permission := principal.PermPolicyRead
	if r.Method != http.MethodGet {
		permission = principal.PermPolicyWrite
	}
	if !s.guard(r.Context(), w, r, permission) {
		return
	}
	var out any
	var err error
	status := http.StatusOK
	switch r.Method {
	case http.MethodGet:
		out, err = s.svc.Policy(r.Context())
	case http.MethodPut:
		var req protocol.PolicyUpdate
		if perr := decodeJSON(bodyFrom(r.Context()), &req); perr != nil {
			writeError(r.Context(), w, perr)
			return
		}
		out, err = s.svc.SavePolicy(r.Context(), req)
	case http.MethodDelete:
		err = s.svc.DiscardPolicy(r.Context())
		out = map[string]bool{"discarded": true}
	case http.MethodPost:
		if !s.guard(r.Context(), w, r, principal.PermServiceManage) {
			return
		}
		var req protocol.DNSApply
		if perr := decodeJSON(bodyFrom(r.Context()), &req); perr != nil {
			writeError(r.Context(), w, perr)
			return
		}
		p, _ := local.PrincipalFrom(r.Context())
		out, err = s.svc.ApplyPolicy(r.Context(), req.Revision, p.String())
		status = http.StatusAccepted
	}
	if r.Method != http.MethodGet {
		s.audit(r.Context(), "policy."+r.Method, "ACL policy", "completed", err)
	}
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, status, out)
}

func (s *Server) handlePolicyCheck(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermPolicyRead) {
		return
	}
	var req protocol.PolicyUpdate
	if err := decodeJSON(bodyFrom(r.Context()), &req); err != nil {
		writeError(r.Context(), w, err)
		return
	}
	if err := s.svc.CheckPolicy(r.Context(), req.Policy); err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}
