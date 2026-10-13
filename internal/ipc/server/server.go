package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"github.com/UruhaLushia/headdeck/internal/service"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
	GoVersion string
	Platform  string
}

type Options struct {
	Service        *service.Service
	Auth           *local.Authenticator
	Logger         *log.Logger
	Authorization  *grants.Store
	Settings       *config.Store
	Reload         func()
	ValidateConfig func(*config.Config) error
	Build          BuildInfo
	SocketPath     string
	StartedAt      time.Time
	Features       []string
}

type Server struct {
	svc            *service.Service
	auth           *local.Authenticator
	logger         *log.Logger
	store          *grants.Store
	settings       *config.Store
	reload         func()
	validateConfig func(*config.Config) error
	build          BuildInfo
	socket         string
	started        time.Time
	features       []string
	mux            *http.ServeMux
}

func New(opts Options) *Server {
	s := &Server{
		svc:      opts.Service,
		auth:     opts.Auth,
		logger:   opts.Logger,
		store:    opts.Authorization,
		settings: opts.Settings, reload: opts.Reload, validateConfig: opts.ValidateConfig,
		build:    opts.Build,
		socket:   opts.SocketPath,
		started:  opts.StartedAt,
		features: opts.Features,
	}
	if s.started.IsZero() {
		s.started = time.Now()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+protocol.PathPing, s.handlePing)
	mux.HandleFunc("POST "+protocol.PathHello, s.handleHello)
	mux.HandleFunc("GET "+protocol.PathStatus, s.handleStatus)
	mux.HandleFunc("GET "+protocol.PathNodes, s.handleNodeList)
	mux.HandleFunc("GET "+protocol.PathNodes+"/{id}", s.handleNodeGet)
	mux.HandleFunc("POST "+protocol.PathNodes+"/{id}/rename", s.handleNodeRename)
	mux.HandleFunc("POST "+protocol.PathNodes+"/{id}/user", s.handleNodeMove)
	mux.HandleFunc("DELETE "+protocol.PathNodes+"/{id}", s.handleNodeDelete)
	mux.HandleFunc("GET "+protocol.PathUsers, s.handleUserList)
	mux.HandleFunc("POST "+protocol.PathUsers, s.handleUserCreate)
	mux.HandleFunc("POST "+protocol.PathUsers+"/{id}/rename", s.handleUserRename)
	mux.HandleFunc("DELETE "+protocol.PathUsers+"/{id}", s.handleUserDelete)
	s.accountRoutes(mux)
	s.loginProviderRoutes(mux)
	s.dnsRoutes(mux)
	s.managedRoutes(mux)
	mux.HandleFunc("GET "+protocol.PathOperations, s.handleOperationList)
	mux.HandleFunc("GET "+protocol.PathOperations+"/{id}", s.handleOperationGet)
	mux.HandleFunc("GET "+protocol.PathHeadscaleService, s.handleHeadscaleService)
	mux.HandleFunc("POST "+protocol.PathHeadscaleService+"/{action}", s.handleHeadscaleAction)
	mux.HandleFunc("GET "+protocol.PathEvents, s.handleEvents)
	mux.HandleFunc("GET "+protocol.PathRoutes, s.handleRouteList)
	mux.HandleFunc("GET "+protocol.PathNodes+"/{id}/routes", s.handleNodeRoutes)
	mux.HandleFunc("POST "+protocol.PathNodes+"/{id}/routes", s.handleNodeRoutesSet)
	mux.HandleFunc("POST "+protocol.PathNodes+"/{id}/tags", s.handleNodeTagsSet)
	mux.HandleFunc("POST "+protocol.PathNodes+"/{id}/expire", s.handleNodeExpire)
	mux.HandleFunc("GET "+protocol.PathPreAuthKeys, s.handlePreAuthKeyList)
	mux.HandleFunc("POST "+protocol.PathPreAuthKeys, s.handlePreAuthKeyCreate)
	mux.HandleFunc("DELETE "+protocol.PathPreAuthKeys+"/{id}", s.handlePreAuthKeyDelete)
	mux.HandleFunc("POST "+protocol.PathPreAuthKeys+"/{id}/expire", s.handlePreAuthKeyExpire)
	mux.HandleFunc("GET "+protocol.PathSettings, s.handleSettingsGet)
	mux.HandleFunc("PATCH "+protocol.PathSettings, s.handleSettingsSet)

	mux.HandleFunc("/", s.handleNotFound)
	s.mux = mux
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := log.WithRequestID(r.Context(), requestID(r))
	r = r.WithContext(ctx)
	if r.Host != protocol.LocalHost || len(r.Header.Values("Origin")) > 0 || len(r.Header.Values("Referer")) > 0 {
		s.reject(ctx, w, nil, r, protocol.NewError(protocol.CodePermissionDenied, "invalid local IPC request"))
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if r.URL.Path == protocol.PathPing {
		s.mux.ServeHTTP(w, r)
		return
	}

	p, perr := s.auth.Authenticate(r, nil)
	if perr != nil {
		s.reject(ctx, w, nil, r, perr)
		return
	}
	body, perr := decodeBody(r, s.auth.MaxBodyBytes)
	if perr != nil {
		s.reject(ctx, w, p, r, perr)
		return
	}

	ctx = withBody(local.WithPrincipal(ctx, p), body)
	s.mux.ServeHTTP(w, r.WithContext(ctx))
}

func (s *Server) reject(ctx context.Context, w http.ResponseWriter, p *principal.Principal, r *http.Request, err *protocol.Error) {
	actor := "anonymous"
	var uid *uint32
	if cred, _ := local.PeerCredFrom(ctx); cred != nil {
		uid = &cred.UID
	}
	if p != nil {
		actor = p.String()
		uid = p.UID
	}
	s.logger.Audit(ctx, log.AuditEvent{
		Actor:     actor,
		Action:    r.Method + " " + r.URL.Path,
		Result:    "denied:" + string(err.Code),
		RequestID: log.RequestID(ctx),
		PeerUID:   uid,
	})
	writeError(ctx, w, err)
}

func (s *Server) audit(ctx context.Context, action, resource, result string, err error) {
	p, _ := local.PrincipalFrom(ctx)
	actor := "anonymous"
	var uid *uint32
	if p != nil {
		actor = p.String()
		uid = p.UID
	}
	ev := log.AuditEvent{
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Result:    result,
		RequestID: log.RequestID(ctx),
		PeerUID:   uid,
	}
	if err != nil {
		ev.Error = err.Error()
	}
	s.logger.Audit(ctx, ev)
}

func (s *Server) guard(ctx context.Context, w http.ResponseWriter, r *http.Request, perm principal.Permission) bool {
	p, ok := local.PrincipalFrom(ctx)
	if !ok {
		writeError(ctx, w, protocol.NewError(protocol.CodeUnauthenticated, "no authenticated principal"))
		return false
	}
	if perr := requirePermission(p, perm); perr != nil {
		s.reject(ctx, w, p, r, perr)
		return false
	}
	return true
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(r.Context(), w, protocol.NewError(protocol.CodeNotFound, "unknown endpoint"))
		return
	}
	writeError(r.Context(), w, protocol.NewError(protocol.CodeMethodNotAllowed,
		"%s is not allowed on %s", r.Method, r.URL.Path))
}

func requestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return sanitizeID(id)
	}
	return "req-" + randomID()
}

func sanitizeID(id string) string {
	if len(id) > 64 {
		id = id[:64]
	}
	var b strings.Builder
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '-', c == '_', c == '.':
			b.WriteByte(c)
		}
	}
	if b.Len() == 0 {
		return "req-" + randomID()
	}
	return b.String()
}
