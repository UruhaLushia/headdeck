package server

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermUsersWrite) {
		return
	}
	var req protocol.CreateUserRequest
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	u, err := s.svc.CreateUser(ctx, req)
	s.audit(ctx, "users.create", req.Name, result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusCreated, u)
}
func (s *Server) handleUserRename(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermUsersWrite) {
		return
	}
	var req protocol.RenameUserRequest
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	id := r.PathValue("id")
	u, err := s.svc.RenameUser(ctx, id, req.NewName)
	s.audit(ctx, "users.rename", id, result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, u)
}
func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermUsersWrite) {
		return
	}
	var req protocol.DeleteUserRequest
	if err := decodeJSON(bodyFrom(ctx), &req); err != nil {
		writeError(ctx, w, err)
		return
	}
	id := r.PathValue("id")
	err := s.svc.DeleteUser(ctx, id, req.ConfirmName)
	s.audit(ctx, "users.delete", id, result(err), err)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}
func result(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
