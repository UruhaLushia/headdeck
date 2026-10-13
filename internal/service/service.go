package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/events"
	"github.com/UruhaLushia/headdeck/internal/headscale"
	hc "github.com/UruhaLushia/headdeck/internal/headscale/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/servicecontrol"
)

type ServiceControl = servicecontrol.Controller

type Service struct {
	controlMu           sync.Mutex
	controlActive       bool
	oidcReady           atomic.Bool
	operationCtx        context.Context
	stopOperations      context.CancelFunc
	operationWG         sync.WaitGroup
	watchWG             sync.WaitGroup
	nodes               resourceSnapshot[protocol.Node]
	users               resourceSnapshot[protocol.User]
	accounts            *accounts.Store
	accountMu           sync.Mutex
	loginProviderReload func(context.Context) error
	cfg                 *config.Config
	dns                 *hc.Store
	hs                  *headscale.Client
	control             ServiceControl
	ops                 *Operations
	hub                 *events.Hub
	started             time.Time

	mu                  sync.Mutex
	snapshot            *protocol.HeadscaleStatus
	probedAt            time.Time
	diagnosticMu        sync.Mutex
	diagnostics         map[string]*protocol.Diagnostic
	metricSamples       []protocol.MetricSample
	headscaleConfigPath string
}

type Options struct {
	DNSDatabase         *sql.DB
	HeadscaleConfigPath string
	HeadscaleManagedDir string
	Accounts            *accounts.Store
	Config              *config.Config
	Control             ServiceControl
	Started             time.Time
	Hub                 *events.Hub
}

func New(opts Options) (*Service, error) {
	if opts.Config == nil {
		return nil, errors.New("service requires a configuration")
	}
	var dns *hc.Store
	if opts.DNSDatabase != nil {
		var err error
		managedDir := opts.HeadscaleManagedDir
		if managedDir == "" {
			managedDir = hc.ManagedDir
		}
		dns, err = hc.NewInDirectory(opts.DNSDatabase, opts.HeadscaleConfigPath, managedDir)
		if err != nil {
			return nil, err
		}
	}
	var socket string
	if opts.DNSDatabase != nil && (opts.Config.Server.Web.Enabled || opts.Control != nil && opts.Control.Driver() == "systemd") {
		var err error
		socket, err = hc.UnixSocket(opts.HeadscaleConfigPath)
		if err != nil {
			return nil, err
		}
	}
	hs, err := headscale.New(headscale.Options{
		BaseURL:            opts.Config.Headscale.BaseURL,
		APIKey:             opts.Config.Headscale.APIKey,
		Timeout:            opts.Config.Headscale.Timeout,
		MaxRetries:         opts.Config.Headscale.MaxRetries,
		InsecureSkipVerify: opts.Config.Headscale.InsecureSkipVerify,
		UnixSocket:         socket,
	})
	if err != nil {
		return nil, err
	}
	started := opts.Started
	if started.IsZero() {
		started = time.Now()
	}
	hub := opts.Hub
	if hub == nil {
		hub = events.NewHub()
	}
	ops := NewOperations()
	operationCtx, stopOperations := context.WithCancel(context.Background())

	s := &Service{
		headscaleConfigPath: opts.HeadscaleConfigPath,
		dns:                 dns,
		operationCtx:        operationCtx,
		stopOperations:      stopOperations,
		accounts:            opts.Accounts,
		cfg:                 opts.Config,
		hs:                  hs,
		control:             opts.Control,
		ops:                 ops,
		hub:                 hub,
		started:             started,
	}
	ops.OnChange(func(op *protocol.Operation) {
		s.publishData(protocol.EventOperationChanged, op.ID, op)
		if op.State != protocol.OperationSucceeded && op.State != protocol.OperationFailed && op.State != protocol.OperationTimeout {
			return
		}
		switch op.Kind {
		case "dns.apply", "config.apply":
			s.publish(protocol.EventDNSChanged, "")
			s.publish(protocol.EventConfigChanged, "")
		case "policy.apply":
			s.publish(protocol.EventPolicyChanged, "")
		case "oidc.apply", "oidc.recover":
			s.publish(protocol.EventConfigChanged, "")
			s.publish(protocol.EventUsersChanged, "")
			s.publish(protocol.EventNodesChanged, "")
		}
	})
	s.watchWG.Add(1)
	go s.watchResources()
	return s, nil
}

func (s *Service) Close() {
	s.controlMu.Lock()
	s.stopOperations()
	s.controlMu.Unlock()
	s.operationWG.Wait()
	s.watchWG.Wait()
	s.hs.Close()
}

func (s *Service) BeginReload() bool {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.controlActive {
		return false
	}
	s.stopOperations()
	return true
}

func (s *Service) Config() *config.Config { return s.cfg }

func (s *Service) Headscale() *headscale.Client { return s.hs }

func (s *Service) Operations() *Operations { return s.ops }

func (s *Service) Hub() *events.Hub { return s.hub }

func (s *Service) publish(eventType, resourceID string) {
	s.hub.Publish(eventType, resourceID)
}

func (s *Service) StartedAt() time.Time { return s.started }
