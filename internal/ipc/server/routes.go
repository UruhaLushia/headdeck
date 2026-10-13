package server

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) handleRouteList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermRoutesRead) {
		return
	}
	routes, err := s.svc.Routes(ctx)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, protocol.RouteList{Routes: routes})
}

func (s *Server) handleNodeRoutes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermRoutesRead) {
		return
	}
	routes, err := s.svc.NodeRoutes(ctx, r.PathValue("id"))
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, protocol.RouteList{Routes: routes})
}

func (s *Server) handleNodeRoutesSet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermRoutesWrite) {
		return
	}
	var req protocol.SetRoutesRequest
	if perr := decodeJSON(bodyFrom(ctx), &req); perr != nil {
		writeError(ctx, w, perr)
		return
	}
	id := r.PathValue("id")
	node, err := s.svc.SetNodeRoutes(ctx, id, req.Routes)
	if err != nil {
		s.audit(ctx, "nodes.routes.set", "node "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "nodes.routes.set", "node "+id, "ok", nil)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleNodeTagsSet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesWrite) {
		return
	}
	var req protocol.SetTagsRequest
	if perr := decodeJSON(bodyFrom(ctx), &req); perr != nil {
		writeError(ctx, w, perr)
		return
	}
	id := r.PathValue("id")
	node, err := s.svc.SetNodeTags(ctx, id, req.Tags)
	if err != nil {
		s.audit(ctx, "nodes.tags.set", "node "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "nodes.tags.set", "node "+id, "ok", nil)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleNodeExpire(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermNodesWrite) {
		return
	}
	id := r.PathValue("id")
	node, err := s.svc.ExpireNode(ctx, id)
	if err != nil {
		s.audit(ctx, "nodes.expire", "node "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "nodes.expire", "node "+id, "ok", nil)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handlePreAuthKeyList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermKeysRead) {
		return
	}
	keys, err := s.svc.PreAuthKeys(ctx, r.URL.Query().Get("user"))
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, protocol.PreAuthKeyList{Keys: keys})
}

func (s *Server) handlePreAuthKeyCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermKeysWrite) {
		return
	}
	var req protocol.CreatePreAuthKeyRequest
	if perr := decodeJSON(bodyFrom(ctx), &req); perr != nil {
		writeError(ctx, w, perr)
		return
	}
	key, err := s.svc.CreatePreAuthKey(ctx, req)
	if err != nil {
		s.audit(ctx, "keys.create", "user "+req.User, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}

	s.audit(ctx, "keys.create", "preauthkey "+key.ID, "ok", nil)
	writeJSON(w, http.StatusOK, key)
}

func (s *Server) handlePreAuthKeyDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermKeysWrite) {
		return
	}
	id := r.PathValue("id")
	if err := s.svc.DeletePreAuthKey(ctx, id); err != nil {
		s.audit(ctx, "keys.delete", "preauthkey "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "keys.delete", "preauthkey "+id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handlePreAuthKeyExpire(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermKeysWrite) {
		return
	}
	id := r.PathValue("id")
	if err := s.svc.ExpirePreAuthKey(ctx, id); err != nil {
		s.audit(ctx, "keys.expire", "preauthkey "+id, "error", err)
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	s.audit(ctx, "keys.expire", "preauthkey "+id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"expired": id})
}
