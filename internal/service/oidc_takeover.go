package service

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"syscall"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	hi "github.com/UruhaLushia/headdeck/internal/headscale/identity"
	"github.com/UruhaLushia/headdeck/internal/installation"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/servicecontrol"
)

func (s *Service) ApplyOIDCTakeover(ctx context.Context, actor string, recover bool) (*protocol.Operation, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || !p.Has(principal.PermAccountsWrite) || !p.Has(principal.PermServiceManage) || p.Kind == principal.KindWebSession {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "login takeover requires a local owner")
	}
	return s.StartOIDCTakeover(actor, recover)
}

func (s *Service) StartOIDCTakeover(actor string, recover bool) (*protocol.Operation, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return nil, err
	}
	if s.control == nil || !s.control.Available() || s.control.Driver() != "systemd" {
		return nil, protocol.NewError(protocol.CodeUnsupported, "login takeover requires systemd service control")
	}
	if os.Geteuid() != 0 {
		return nil, protocol.NewError(protocol.CodeUnsupported, "login takeover requires the privileged daemon")
	}
	if !recover {
		if !s.cfg.Server.Web.Enabled || !s.cfg.Server.Web.OIDCServer.Enabled {
			return nil, protocol.NewError(protocol.CodeClientConfig, "configure HTTPS and enable Headdeck Web first")
		}
		u, err := url.Parse(s.cfg.Headscale.BaseURL)
		if err != nil {
			return nil, err
		}
		ip, err := netip.ParseAddr(u.Hostname())
		if u.Hostname() != "localhost" && (err != nil || !ip.IsLoopback()) {
			return nil, protocol.NewError(protocol.CodeUnsupported, "login takeover requires a local Headscale API")
		}
	}
	kind := "oidc.apply"
	if recover {
		kind = "oidc.recover"
	}
	op := s.ops.Start(kind, s.control.Unit(), actor)
	s.controlActive = true
	s.oidcReady.Store(false)
	s.operationWG.Add(1)
	go s.runOIDCTakeover(op.ID, recover)
	return op, nil
}

func (s *Service) prepareOIDCTakeover(ctx context.Context) (*hi.Journal, error) {
	if err := servicecontrol.CheckHeadscaleEnvironment(ctx, s.control.Unit()); err != nil {
		return nil, err
	}
	change, err := s.dns.PlanOIDC(s.cfg.Server.Web)
	if err != nil {
		return nil, err
	}
	state, err := s.control.State(ctx)
	if err != nil {
		return nil, err
	}
	if state.ActiveState != "active" || state.MainPID == 0 {
		return nil, fmt.Errorf("start Headscale before taking over login")
	}
	if err := installation.CheckHeadscaleArguments(state.MainPID); err != nil {
		return nil, err
	}
	info, err := os.Stat(fmt.Sprintf("/proc/%d", state.MainPID))
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot determine the Headscale service user")
	}
	bindings, err := s.oidcBindings(ctx)
	if err != nil {
		return nil, err
	}
	return &hi.Journal{Configuration: *change, Bindings: bindings, UID: stat.Uid, Unit: s.control.Unit()}, nil
}

func (s *Service) stopOIDCService(ctx context.Context) error {
	if _, err := s.control.Action(ctx, s.control.Unit(), protocol.ServiceStop); err != nil {
		return err
	}
	state, err := s.control.State(ctx)
	if err != nil {
		return err
	}
	if state.ActiveState != "inactive" || state.MainPID != 0 {
		return fmt.Errorf("headscale must be stopped before updating identities")
	}
	return nil
}

func (s *Service) startOIDCService(ctx context.Context, id string) error {
	if _, err := s.control.Action(ctx, s.control.Unit(), protocol.ServiceStart); err != nil {
		return err
	}
	return s.waitForHeadscale(ctx, id)
}
