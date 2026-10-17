package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/github"
	"github.com/UruhaLushia/headdeck/internal/auth/oidc"
	"github.com/UruhaLushia/headdeck/internal/auth/oidcserver"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"github.com/UruhaLushia/headdeck/internal/service"
)

const (
	sessionCookie = "headdeck_session"
	// Rate limits avoid an attacker triggering account lockout.
	maxLoginAttemptsPerMinute = 10
)

type Server struct {
	cfg            config.WebConfig
	svc            *service.Service
	logger         *log.Logger
	sessions       *sessionStore
	limiter        *attemptLimiter
	mux            *http.ServeMux
	now            func() time.Time
	settings       *config.Store
	validateConfig func(*config.Config) error
	authorizeLogin func(context.Context) error
	reload         func()
	providers      map[string]accounts.Provider
	flows          loginFlows
	credentials    *accounts.Store
	authStates     authStates
	securityKey    []byte
	providerMu     sync.RWMutex
	oidcServer     *oidcserver.Provider
	headscaleProxy http.Handler
	proxyTransport *http.Transport
	proxyContext   context.Context
	stopProxy      context.CancelFunc
}

type Options struct {
	Config  config.WebConfig
	Service *service.Service
	Logger  *log.Logger
	// Rotating the 0600 session key invalidates all sessions.
	SessionKeyFile string
	Settings       *config.Store
	ValidateConfig func(*config.Config) error
	AuthorizeLogin func(context.Context) error
	Reload         func()
}

func New(opts Options) (*Server, error) {
	if opts.Service == nil {
		return nil, errors.New("the web listener needs the service layer")
	}
	if opts.AuthorizeLogin == nil {
		return nil, errors.New("web login requires Headscale takeover validation")
	}
	if opts.SessionKeyFile == "" {
		opts.SessionKeyFile = opts.Config.SessionKeyFile
	}
	key, err := loadOrCreateSessionKey(opts.SessionKeyFile)
	if err != nil {
		return nil, err
	}
	if opts.Settings == nil {
		return nil, errors.New("web sessions require the settings store")
	}
	sessions, err := newSessionStore(opts.Settings.Database(), key, opts.Config.SessionTTL)
	if err != nil {
		return nil, err
	}
	credentials, err := accounts.New(opts.Settings.Database())
	if err != nil {
		return nil, err
	}
	securityKey, err := loadOrCreateSessionKey(opts.SessionKeyFile + ".auth")
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:            opts.Config,
		svc:            opts.Service,
		logger:         opts.Logger,
		sessions:       sessions,
		limiter:        newAttemptLimiter(maxLoginAttemptsPerMinute, time.Minute),
		now:            time.Now,
		providers:      map[string]accounts.Provider{},
		credentials:    credentials,
		securityKey:    securityKey,
		settings:       opts.Settings,
		validateConfig: opts.ValidateConfig,
		authorizeLogin: opts.AuthorizeLogin,
		reload:         opts.Reload,
	}
	if opts.Config.OIDC.Enabled {
		s.providers["oidc"] = oidc.New(opts.Config.OIDC.Issuer, opts.Config.OIDC.ClientID, opts.Config.OIDC.ClientSecret)
	}
	if opts.Config.GitHub.Enabled {
		s.providers["github"] = github.New(opts.Config.GitHub.ClientID, opts.Config.GitHub.ClientSecret)
	}
	if err := s.loadProviders(context.Background()); err != nil {
		return nil, err
	}
	s.svc.SetLoginProviderReload(s.loadProviders)
	if opts.Config.HeadscaleProxy {
		if err := s.initHeadscaleProxy(); err != nil {
			return nil, err
		}
	}
	mux := http.NewServeMux()
	if err := s.oidcRoutes(mux); err != nil {
		s.Close()
		return nil, err
	}
	mux.HandleFunc("GET /api/auth/methods", s.handleLoginMethods)
	for _, name := range []string{"oidc", "github"} {
		mux.HandleFunc("GET /api/auth/"+name+"/start", func(w http.ResponseWriter, r *http.Request) { s.handleProviderStart(w, r, name) })
		mux.HandleFunc("GET /api/auth/"+name+"/callback", func(w http.ResponseWriter, r *http.Request) { s.handleProviderCallback(w, r, name) })
	}
	mux.HandleFunc("POST /api/session", s.handleLogin)
	mux.HandleFunc("DELETE /api/session", s.handleLogout)
	mux.HandleFunc("GET /api/session", s.handleSession)
	s.sessionDevices(mux)
	s.securityRoutes(mux)
	s.providerRoutes(mux)
	mux.HandleFunc("GET /api/status", s.withSession(s.requirePermission(principal.PermSystemRead, s.handleStatus)))
	mux.HandleFunc("GET /api/logs/{source}", s.withSession(s.requirePermission(principal.PermAuditRead, s.handleLogs)))
	mux.HandleFunc("GET /api/headscale/diagnostics/{section}", s.withSession(s.requirePermission(principal.PermSystemRead, s.withDiagnostics(s.handleDiagnostic))))
	mux.HandleFunc("GET /api/debug/pprof/", s.withSession(s.requirePermission(principal.PermSystemRead, s.withDiagnostics(s.handleDebug))))
	mux.HandleFunc("GET /api/status/stream", s.withSession(s.requirePermission(principal.PermSystemRead, s.handleStatusStream)))
	mux.HandleFunc("GET /api/nodes", s.withSession(s.requirePermission(principal.PermNodesRead, s.handleNodes)))
	mux.HandleFunc("GET /api/users", s.withSession(s.requirePermission(principal.PermUsersRead, s.handleUsers)))
	mux.HandleFunc("GET /api/routes", s.withSession(s.requirePermission(principal.PermRoutesRead, s.handleRoutes)))
	mux.HandleFunc("GET /api/preauth-keys", s.withSession(s.requirePermission(principal.PermKeysRead, s.handleKeys)))
	mux.HandleFunc("GET /api/events", s.withSession(s.requirePermission(principal.PermSystemRead, s.handleEvents)))
	mux.HandleFunc("GET /", s.handleStatic)
	s.nodeRoutes(mux)
	s.userRoutes(mux)
	s.managedRoutes(mux)
	s.settingsRoutes(mux)
	s.mux = mux
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.headscaleProxy != nil && !headdeckRoute(r.URL.Path) {
		s.serveHeadscale(w, r)
		return
	}
	request, ok := s.routeRequest(w, r)
	if !ok {
		return
	}
	r = request
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
	}
	// Cross-site requests cannot set this header without a preflight.
	if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet &&
		r.Method != http.MethodHead {
		if r.Header.Get("X-Requested-With") != "headdeck" {
			s.writeError(w, protocol.NewError(protocol.CodePermissionDenied,
				"missing X-Requested-With header"))
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && !s.originAllowed(origin) {
			s.writeError(w, protocol.NewError(protocol.CodePermissionDenied,
				"cross-origin request rejected"))
			return
		}
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	if strings.HasPrefix(r.URL.Path, "/api/") && (r.Method != http.MethodDelete || r.URL.Path != "/api/session") {
		if err := s.authorizeLogin(r.Context()); err != nil {
			s.writeError(w, protocol.NewError(protocol.CodeDaemonUnavailable, "Headscale management is not ready; check daemon operations"))
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Close() {
	if s.stopProxy != nil {
		s.stopProxy()
		s.proxyTransport.CloseIdleConnections()
	}
}
