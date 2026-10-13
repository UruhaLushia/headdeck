package server

import (
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) handleNodeMove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesWrite) {
		return
	}
	var req protocol.MoveNodeRequest
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	id := r.PathValue("id")
	n, err := s.svc.MoveNode(ctx, id, req.UserID)
	s.audit(ctx, "nodes.move", id, result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, n)
}
