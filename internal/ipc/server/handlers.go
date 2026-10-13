package server

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/processinfo"
)

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, protocol.Ping{Service: "headdeck"})
}

func (s *Server) handleHello(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, ok := local.PrincipalFrom(ctx)
	if !ok {
		writeError(ctx, w, protocol.NewError(protocol.CodeUnauthenticated, "no authenticated principal"))
		return
	}
	var req protocol.HelloRequest
	if perr := decodeJSON(bodyFrom(ctx), &req); perr != nil {
		s.reject(ctx, w, p, r, perr)
		return
	}
	version, ok := protocol.Negotiate(
		protocol.VersionRange{Min: req.ProtocolMin, Max: req.ProtocolMax},
		protocol.SupportedVersions,
	)
	if !ok {
		s.audit(ctx, "system.hello", "", "rejected:protocol", nil)
		writeError(ctx, w, protocol.NewError(protocol.CodeProtocolIncompatible,
			"client protocol %d-%d does not intersect daemon protocol %s",
			req.ProtocolMin, req.ProtocolMax, protocol.SupportedVersions).
			WithDetail("daemon_protocol_min", strconv.Itoa(protocol.SupportedVersions.Min)).
			WithDetail("daemon_protocol_max", strconv.Itoa(protocol.SupportedVersions.Max)))
		return
	}
	s.audit(ctx, "system.hello", "protocol:"+strconv.Itoa(version), "ok", nil)
	writeJSON(w, http.StatusOK, protocol.HelloResponse{
		ServerVersion:   s.build.Version,
		ProtocolVersion: version,
		ProtocolMin:     protocol.SupportedVersions.Min,
		ProtocolMax:     protocol.SupportedVersions.Max,
		Features:        s.featuresFor(p),
		Principal:       p.String(),
		Role:            string(p.Role),
		ServerTime:      time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) featuresFor(p *principal.Principal) []string {
	allowed := principal.FeaturesFor(p.PermissionList())
	if p.Has(principal.PermSystemRead) {
		allowed = append(allowed, protocol.FeatureEventsSSE)
	}
	var out []string
	for _, compiled := range s.features {
		for _, feature := range allowed {
			if feature == compiled {
				out = append(out, feature)
				break
			}
		}
	}
	return out
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !s.guard(ctx, w, r, principal.PermSystemRead) {
		return
	}
	force := r.URL.Query().Get("refresh") != ""
	status, err := s.svc.Status(ctx, force)
	if err != nil {
		writeError(ctx, w, protocol.FromError(err))
		return
	}
	status.Server = protocol.ServerStatus{
		State:            "running",
		RSSBytes:         processinfo.ResidentMemory(pid()),
		Version:          s.build.Version,
		Commit:           s.build.Commit,
		BuildTime:        s.build.BuildTime,
		GoVersion:        s.build.GoVersion,
		Platform:         s.build.Platform,
		StartedAt:        s.started.UTC().Format(time.RFC3339),
		Uptime:           time.Since(s.started).Truncate(time.Second).String(),
		PID:              pid(),
		Socket:           s.socket,
		EventSubscribers: s.svc.Hub().Subscribers(),
	}
	status.ProtocolVersion = protocol.ProtocolVersion
	status.CompiledFeatures = s.features
	if p, ok := local.PrincipalFrom(ctx); ok {
		perms := make([]string, 0, len(p.PermissionList()))
		for _, perm := range p.PermissionList() {
			perms = append(perms, string(perm))
		}
		status.Principal = protocol.PrincipalStatus{
			Kind:        string(p.Kind),
			ID:          p.ID,
			Role:        string(p.Role),
			Permissions: perms,
		}
	}
	status.Authorization = s.authorizationStatus()
	s.audit(ctx, "system.status", "", "ok", nil)
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) authorizationStatus() protocol.Authorization {
	users, err := s.store.Read()
	out := protocol.Authorization{Path: s.store.Path, Users: len(users), MissingFile: errors.Is(err, os.ErrNotExist)}
	if err != nil {
		out.Error = err.Error()
	}
	return out
}
