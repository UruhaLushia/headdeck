package service

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) ManagedConfig(ctx context.Context) (*protocol.ManagedConfig, error) {
	if s.dns == nil {
		return nil, protocol.NewError(protocol.CodeUnsupported, "Headscale configuration management is unavailable")
	}
	return s.dns.Managed(ctx)
}

func (s *Service) SaveManagedConfig(ctx context.Context, req protocol.ManagedConfigUpdate) (*protocol.ManagedConfig, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return nil, err
	}
	out, err := s.dns.SaveManaged(ctx, req)
	if err == nil {
		s.publish(protocol.EventConfigChanged, "")
		s.publish(protocol.EventDNSChanged, "")
	}
	return out, err
}

func (s *Service) DiscardManagedConfig(ctx context.Context) error {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return err
	}
	if err := s.dns.DiscardManaged(ctx); err != nil {
		return err
	}
	s.publish(protocol.EventConfigChanged, "")
	s.publish(protocol.EventDNSChanged, "")
	return nil
}

func (s *Service) configurationWritable() error {
	if s.operationCtx.Err() != nil {
		return protocol.NewError(protocol.CodeDaemonUnavailable, "daemon is shutting down")
	}
	if s.controlActive {
		return protocol.NewError(protocol.CodeOperationInProgress, "wait for the Headscale operation to finish")
	}
	if s.dns == nil {
		return protocol.NewError(protocol.CodeUnsupported, "Headscale configuration management is unavailable")
	}
	return nil
}

func (s *Service) ApplyManagedConfig(ctx context.Context, revision, actor string) (*protocol.Operation, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return nil, err
	}
	out, err := s.dns.Managed(ctx)
	if err != nil {
		return nil, err
	}
	if revision != out.Revision {
		return nil, protocol.NewError(protocol.CodeConflict, "Headscale settings changed; refresh before applying")
	}
	if !out.Pending || !out.Writable {
		return nil, protocol.NewError(protocol.CodeBadRequest, "no writable pending Headscale changes")
	}
	if _, err := s.HeadscaleService(ctx); err != nil {
		return nil, err
	}
	op := s.ops.Start("config.apply", s.control.Unit(), actor)
	s.controlActive = true
	s.operationWG.Add(1)
	go s.runConfiguration(op.ID, "Headscale configuration", func(ctx context.Context) (configApplication, error) {
		applied, err := s.dns.ApplyManaged(ctx, revision)
		if err != nil {
			return configApplication{}, err
		}
		return configApplication{restart: applied.RestartRequired, finish: applied.Finish}, nil
	})
	return op, nil
}
