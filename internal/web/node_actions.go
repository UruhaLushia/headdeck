package web

import (
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) nodeRoutes(mux *http.ServeMux) {
	write := []principal.Permission{principal.PermNodesWrite}
	manage := []principal.Permission{principal.PermNodesManage}
	s.endpoint(mux, "POST /api/nodes/register", manage, withBody(func(r *http.Request, v protocol.RegisterNodeRequest) (any, error) {
		return s.svc.RegisterNode(r.Context(), v)
	}))
	s.endpoint(mux, "POST /api/nodes/{id}/rename", manage, withBody(func(r *http.Request, v protocol.RenameNodeRequest) (any, error) {
		return s.svc.RenameNode(r.Context(), r.PathValue("id"), v.NewName)
	}))
	s.endpoint(mux, "POST /api/nodes/{id}/tags", write, withBody(func(r *http.Request, v protocol.SetTagsRequest) (any, error) {
		return s.svc.SetNodeTags(r.Context(), r.PathValue("id"), v.Tags)
	}))
	s.endpoint(mux, "POST /api/nodes/{id}/owner", write, withBody(func(r *http.Request, v protocol.MoveNodeRequest) (any, error) {
		return s.svc.MoveNode(r.Context(), r.PathValue("id"), v.UserID)
	}))
	s.endpoint(mux, "POST /api/nodes/{id}/expire", manage, func(r *http.Request) (any, error) { return s.svc.ExpireNode(r.Context(), r.PathValue("id")) })
	s.endpoint(mux, "POST /api/nodes/{id}/routes", []principal.Permission{principal.PermRoutesWrite}, withBody(func(r *http.Request, v protocol.SetRoutesRequest) (any, error) {
		return s.svc.SetNodeRoutes(r.Context(), r.PathValue("id"), v.Routes)
	}))
	s.endpoint(mux, "DELETE /api/nodes/{id}", []principal.Permission{principal.PermNodesDelete}, withBody(func(r *http.Request, v protocol.DeleteNodeRequest) (any, error) {
		return map[string]bool{"deleted": true}, s.svc.DeleteNode(r.Context(), r.PathValue("id"), v.ConfirmName)
	}))
}
