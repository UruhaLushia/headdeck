package server

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) dnsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+protocol.PathDNS, s.handleDNS)
	mux.HandleFunc("PUT "+protocol.PathDNS, s.handleDNSSave)
	mux.HandleFunc("DELETE "+protocol.PathDNS, s.handleDNSDiscard)
	mux.HandleFunc("POST "+protocol.PathDNS+"/apply", s.handleDNSApply)
}

func (s *Server) handleDNS(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermDNSRead) {
		return
	}
	out, err := s.svc.DNS(r.Context())
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDNSSave(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermDNSWrite) {
		return
	}
	var req protocol.DNSUpdate
	if err := decodeJSON(bodyFrom(r.Context()), &req); err != nil {
		writeError(r.Context(), w, err)
		return
	}
	out, err := s.svc.SaveDNS(r.Context(), req)
	s.audit(r.Context(), "dns.save", "DNS configuration", "completed", err)
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDNSDiscard(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermDNSWrite) {
		return
	}
	err := s.svc.DiscardDNS(r.Context())
	s.audit(r.Context(), "dns.discard", "DNS draft", "completed", err)
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"discarded": true})
}

func (s *Server) handleDNSApply(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermDNSWrite) || !s.guard(r.Context(), w, r, principal.PermServiceManage) {
		return
	}
	var req protocol.DNSApply
	if err := decodeJSON(bodyFrom(r.Context()), &req); err != nil {
		writeError(r.Context(), w, err)
		return
	}
	p, _ := local.PrincipalFrom(r.Context())
	op, err := s.svc.ApplyDNS(r.Context(), req.Revision, p.String())
	s.audit(r.Context(), "dns.apply", "DNS configuration", "accepted", err)
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusAccepted, op)
}
