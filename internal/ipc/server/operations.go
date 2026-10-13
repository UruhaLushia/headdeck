package server

import (
	"net/http"
	"os"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
)

func (s *Server) handleOperationList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermOperationsRead) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operations": s.svc.Operations().List()})
}

func (s *Server) handleOperationGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermOperationsRead) {
		return
	}
	op, ok := s.svc.Operations().Get(r.PathValue("id"))
	if !ok {
		writeError(ctx, w, protocol.NewError(protocol.CodeNotFound, "unknown operation"))
		return
	}
	writeJSON(w, http.StatusOK, op)
}

func (s *Server) handleHeadscaleService(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermSystemRead) {
		return
	}
	state, err := s.svc.HeadscaleService(r.Context())
	if err != nil {
		writeError(r.Context(), w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleHeadscaleAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermServiceManage) {
		return
	}
	var req protocol.ServiceActionRequest
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	if len(req.Reason) > 512 {
		writeError(ctx, w, protocol.NewError(protocol.CodeBadRequest, "reason exceeds 512 bytes"))
		return
	}
	p, _ := local.PrincipalFrom(ctx)
	action := protocol.ServiceAction(r.PathValue("action"))
	op, err := s.svc.StartHeadscaleAction(ctx, action, p.String())
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.logger.Audit(ctx, log.AuditEvent{Actor: p.String(), PeerUID: p.UID,
		Action: "headscale." + string(action), Resource: op.Target, Result: "accepted",
		OperationID: op.ID, Reason: strings.TrimSpace(req.Reason)})
	writeJSON(w, http.StatusAccepted, op)
}

func pid() int { return os.Getpid() }
