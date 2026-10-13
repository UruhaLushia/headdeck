package server

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/service"
)

func (s *Server) handleNodeList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesRead) {
		return
	}
	user := r.URL.Query().Get("user")
	nodes, err := s.svc.NodeSnapshot(ctx, user)
	if err != nil {
		s.audit(ctx, "nodes.list", user, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleNodeGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesRead) {
		return
	}
	id := r.PathValue("id")
	node, err := s.svc.Node(ctx, id)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleNodeRename(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesWrite) {
		return
	}
	var req protocol.RenameNodeRequest
	if perr := decodeJSON(bodyFrom(ctx), &req); perr != nil {
		writeError(ctx, w, perr)
		return
	}
	id := r.PathValue("id")
	node, err := s.svc.RenameNode(ctx, id, req.NewName)
	if err != nil {
		s.audit(ctx, "nodes.rename", "node "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "nodes.rename", service.Describe(*node), "ok", nil)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleNodeDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesDelete) {
		return
	}
	var req protocol.DeleteNodeRequest
	if body := bodyFrom(ctx); len(body) > 0 {
		if perr := decodeJSON(body, &req); perr != nil {
			writeError(ctx, w, perr)
			return
		}
	}
	id := r.PathValue("id")
	if err := s.svc.DeleteNode(ctx, id, req.ConfirmName); err != nil {
		s.audit(ctx, "nodes.delete", "node "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "nodes.delete", "node "+id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleUserList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermUsersRead) {
		return
	}
	users, err := s.svc.UserSnapshot(ctx)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, users)
}
