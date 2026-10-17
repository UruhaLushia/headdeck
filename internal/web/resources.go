package web

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/processinfo"
	"github.com/UruhaLushia/headdeck/internal/version"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	p, _ := principalFrom(r)
	status, err := s.status(r.Context(), p, r.URL.Query().Get("refresh") != "")
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleDiagnostic(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Diagnostic(r.Context(), r.PathValue("section"))
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Logs(r.Context(), r.PathValue("source"))
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) status(ctx context.Context, p *principal.Principal, refresh bool) (*protocol.Status, error) {
	status, err := s.svc.Status(ctx, refresh)
	if err != nil {
		return nil, err
	}
	status.ProtocolVersion = protocol.ProtocolVersion
	build := version.Get()
	status.Server = protocol.ServerStatus{State: "running", RSSBytes: processinfo.ResidentMemory(os.Getpid()), Version: build.Version, Commit: build.Commit, BuildTime: build.BuildTime, GoVersion: build.GoVersion, Platform: build.Platform, PID: os.Getpid(), StartedAt: s.svc.StartedAt().UTC().Format(time.RFC3339), Uptime: time.Since(s.svc.StartedAt()).Round(time.Second).String(), EventSubscribers: s.svc.Hub().Subscribers()}
	status.CompiledFeatures = protocol.AllFeatures
	if p != nil {
		status.Principal = protocol.PrincipalStatus{
			Kind: string(p.Kind), ID: p.ID, Role: string(p.Role),
			Permissions: permissionStrings(p),
		}
	}
	return status, nil
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	if p, ok := principalFrom(r); ok && p.Role == principal.RoleMember {
		user = ""
	}
	nodes, err := s.svc.NodeSnapshot(r.Context(), user)
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	if p, ok := principalFrom(r); ok && p.Role == principal.RoleMember {
		nodes.Nodes = ownedItems(nodes.Nodes, p.HeadscaleUserID, func(n protocol.Node) string { return n.User.ID })
		nodes.Revision = 0
	}
	s.writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.UserSnapshot(r.Context())
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	if p, ok := principalFrom(r); ok && p.Role == principal.RoleMember {
		users.Users = ownedItems(users.Users, p.HeadscaleUserID, func(u protocol.User) string { return u.ID })
		users.Revision = 0
	}
	s.writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := s.svc.Routes(r.Context())
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	if p, ok := principalFrom(r); ok && p.Role == principal.RoleMember {
		routes = ownedItems(routes, p.HeadscaleUserID, func(route protocol.Route) string { return route.User.ID })
	}
	s.writeJSON(w, http.StatusOK, protocol.RouteList{Routes: routes})
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	if p, ok := principalFrom(r); ok && p.Role == principal.RoleMember {
		user = ""
	}
	keys, err := s.svc.PreAuthKeys(r.Context(), user)
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	if p, ok := principalFrom(r); ok && p.Role == principal.RoleMember {
		keys = ownedItems(keys, p.HeadscaleUserID, func(key protocol.PreAuthKey) string { return key.User.ID })
	}
	s.writeJSON(w, http.StatusOK, protocol.PreAuthKeyList{Keys: keys})
}

func ownedItems[T any](items []T, userID string, owner func(T) string) []T {
	owned := make([]T, 0, len(items))
	for _, item := range items {
		if userID != "" && owner(item) == userID {
			owned = append(owned, item)
		}
	}
	return owned
}
