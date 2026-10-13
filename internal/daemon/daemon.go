package daemon

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/auth/local"
	"github.com/UruhaLushia/headdeck/internal/auth/oidcserver"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/events"
	hc "github.com/UruhaLushia/headdeck/internal/headscale/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/ipc/server"
	"github.com/UruhaLushia/headdeck/internal/ipc/transport"
	"github.com/UruhaLushia/headdeck/internal/log"
	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/UruhaLushia/headdeck/internal/service"
	"github.com/UruhaLushia/headdeck/internal/servicecontrol"
	"github.com/UruhaLushia/headdeck/internal/version"
	"github.com/UruhaLushia/headdeck/internal/web"
)

var ErrReload = errors.New("reload daemon settings")

type Options struct {
	HeadscaleConfigPath string
	Config              *config.Config
	ConfigPath          string
	Authorization       *grants.Store
	LogLevel            string
	LogJSON             bool
	Logger              *log.Logger
	Stdout              *os.File
}

type Daemon struct {
	cfg      *config.Config
	logger   *log.Logger
	store    *grants.Store
	settings *config.Store
	reload   chan struct{}
	auth     *local.Authenticator
	svc      *service.Service
	handler  *server.Server
	web      *web.Server
	started  time.Time
}

func New(opts Options) (*Daemon, error) {
	cfg := opts.Config
	if cfg == nil {
		return nil, errors.New("daemon requires a configuration")
	}
	control := servicecontrol.New(cfg)
	if cfg.Server.Web.Enabled || control.Driver() == "systemd" {
		webListen := ""
		if cfg.Server.Web.Enabled {
			webListen = cfg.Server.Web.Listen
		}
		upstream, err := hc.Upstream(opts.HeadscaleConfigPath, webListen)
		if err != nil {
			return nil, fmt.Errorf("headscale listener: %w", err)
		}
		cfg.Headscale.BaseURL = upstream
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(log.Options{Level: opts.LogLevel, JSON: opts.LogJSON})
	}
	store := opts.Authorization
	if store == nil {
		store = &grants.Store{Path: paths.AuthorizedUsers}
	}
	settings, err := config.Open(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	accountStore, err := accounts.New(settings.Database())
	if err != nil {
		settings.Close()
		return nil, err
	}
	hub := events.NewHub()
	svc, err := service.New(service.Options{Config: cfg, Control: control, Hub: hub, Accounts: accountStore, DNSDatabase: settings.Database(), HeadscaleConfigPath: opts.HeadscaleConfigPath})
	if err != nil {
		settings.Close()
		return nil, err
	}
	info := version.Get()
	started := time.Now()
	d := &Daemon{
		cfg:     cfg,
		logger:  logger,
		store:   store,
		svc:     svc,
		started: started,
	}
	d.reload = make(chan struct{}, 1)
	d.auth = &local.Authenticator{Authorized: store, MaxBodyBytes: 1 << 20}
	d.settings = settings

	d.handler = server.New(server.Options{
		Service:        svc,
		Auth:           d.auth,
		Logger:         logger,
		Authorization:  store,
		Settings:       settings,
		Reload:         d.requestReload,
		ValidateConfig: func(next *config.Config) error { return validateSettings(next, cfg, opts.HeadscaleConfigPath) },
		Build: server.BuildInfo{
			Version:   info.Version,
			Commit:    info.Commit,
			BuildTime: info.BuildTime,
			GoVersion: info.GoVersion,
			Platform:  info.Platform,
		},
		SocketPath: cfg.IPC.Listen,
		StartedAt:  started,
		Features:   protocol.AllFeatures,
	})
	if cfg.Server.Web.Enabled {
		webSrv, err := web.New(web.Options{
			Config:         cfg.Server.Web,
			Service:        svc,
			Logger:         logger,
			SessionKeyFile: cfg.Server.Web.SessionKeyFile,
			Settings:       settings,
			ValidateConfig: func(next *config.Config) error { return validateSettings(next, cfg, opts.HeadscaleConfigPath) },
			AuthorizeLogin: svc.CheckOIDCTakeover,
			Reload:         d.requestReload,
		})
		if err != nil {
			svc.Close()
			settings.Close()
			return nil, fmt.Errorf("web listener: %w", err)
		}
		d.web = webSrv
	}
	return d, nil
}

func (d *Daemon) requestReload() {
	select {
	case d.reload <- struct{}{}:
	default:
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	defer d.settings.Close()
	defer d.svc.Close()
	if d.web != nil {
		defer d.web.Close()
	}
	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	mode, err := d.cfg.SocketFileMode()
	if err != nil {
		return err
	}
	ln, err := transport.Listen(ctx, transport.ListenOptions{
		Path:        d.cfg.IPC.Listen,
		Mode:        mode,
		DirMode:     0o755,
		RemoveStale: true,
	})
	if err != nil {
		return err
	}
	defer ln.Close()

	logger := d.logger.Named("ipc")
	logger.Info("listening",
		"path", d.cfg.IPC.Listen,
		"uid", os.Getuid(),
		"headscale", d.cfg.Headscale.BaseURL,
		"authorization", d.store.Path)

	srv := &http.Server{
		Handler:           d.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ConnContext:       local.ConnContext,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          nil,
	}
	defer srv.Close()

	errCh := make(chan error, 2)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var webSrv *http.Server
	if d.web != nil {
		webLn, err := net.Listen("tcp", d.cfg.Server.Web.Listen)
		if err != nil {
			return fmt.Errorf("web listener: %w", err)
		}
		defer webLn.Close()
		webSrv = &http.Server{
			Addr:              d.cfg.Server.Web.Listen,
			Handler:           d.web,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			BaseContext:       func(net.Listener) context.Context { return ctx },
		}
		defer webSrv.Close()
		logger.Info("web listener enabled",
			"listen", d.cfg.Server.Web.Listen,
			"tls", d.cfg.Server.Web.TLSCert != "",
			"cookie_secure", d.cfg.Server.Web.CookieSecure)
		go func() {
			var err error
			if d.cfg.Server.Web.TLSCert != "" {
				err = webSrv.ServeTLS(webLn, d.cfg.Server.Web.TLSCert, d.cfg.Server.Web.TLSKey)
			} else {
				err = webSrv.Serve(webLn)
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("web listener: %w", err)
			}
		}()
		if _, err := d.svc.StartOIDCTakeover("headdeck", false); err != nil {
			logger.Error("Headscale management could not start", "error", err)
		}
	}

	var result error
	reloadCheck := time.NewTimer(0)
	reloadCheck.Stop()
	defer reloadCheck.Stop()

	for {
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("ipc server: %w", err)
			}
			return nil
		case <-d.reload:
			reloadCheck.Reset(0)
		case <-reloadCheck.C:
			if d.svc.BeginReload() {
				result = ErrReload
				goto shutdown
			}
			reloadCheck.Reset(100 * time.Millisecond)
		case <-ctx.Done():
			goto shutdown
		}
	}

shutdown:
	cancelRun()
	if d.web != nil {
		d.web.Close()
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful shutdown failed", "error", err.Error())
	}
	if webSrv != nil {
		if err := webSrv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("web shutdown failed", "error", err.Error())
		}
	}
	return result
}

func validateSettings(c, current *config.Config, headscalePath string) error {
	if c.Server.Web.OIDCServer.Enabled {
		if err := oidcserver.ValidateKey(c.Server.Web.OIDCServer.SigningKeyFile); err != nil {
			return fmt.Errorf("OIDC signing key: %w", err)
		}
	}
	if c.Server.Web.Enabled {
		if _, err := hc.Upstream(headscalePath, c.Server.Web.Listen); err != nil {
			return fmt.Errorf("headscale listener: %w", err)
		}
		if c.Server.Web.TLSCert != "" {
			if _, err := tls.LoadX509KeyPair(c.Server.Web.TLSCert, c.Server.Web.TLSKey); err != nil {
				return fmt.Errorf("web TLS files could not be loaded")
			}
		}
		if !current.Server.Web.Enabled || current.Server.Web.Listen != c.Server.Web.Listen {
			listener, err := net.Listen("tcp", c.Server.Web.Listen)
			if err != nil {
				return fmt.Errorf("web listener unavailable: %w", err)
			}
			listener.Close()
		}

		if err := web.ValidateSessionKey(c.Server.Web.SessionKeyFile); err != nil {
			return fmt.Errorf("web settings: %w", err)
		}
	}
	return nil
}
