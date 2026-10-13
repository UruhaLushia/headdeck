package service

import (
	"context"
	"errors"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) DNS(ctx context.Context) (*protocol.DNSSettings, error) {
	if s.dns == nil {
		return nil, protocol.NewError(protocol.CodeUnsupported, "DNS configuration management is unavailable")
	}
	return s.dns.Get(ctx)
}

func (s *Service) SaveDNS(ctx context.Context, req protocol.DNSUpdate) (*protocol.DNSSettings, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.controlActive {
		return nil, protocol.NewError(protocol.CodeOperationInProgress, "wait for the Headscale operation to finish")
	}
	if s.dns == nil {
		return nil, protocol.NewError(protocol.CodeUnsupported, "DNS configuration management is unavailable")
	}
	out, err := s.dns.Save(ctx, req)
	if err == nil {
		s.publish(protocol.EventDNSChanged, "")
		s.publish(protocol.EventConfigChanged, "")
	}
	return out, err
}

func (s *Service) DiscardDNS(ctx context.Context) error {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.controlActive {
		return protocol.NewError(protocol.CodeOperationInProgress, "wait for the Headscale operation to finish")
	}
	if s.dns == nil {
		return protocol.NewError(protocol.CodeUnsupported, "DNS configuration management is unavailable")
	}
	if err := s.dns.Discard(ctx); err != nil {
		return err
	}
	s.publish(protocol.EventDNSChanged, "")
	s.publish(protocol.EventConfigChanged, "")
	return nil
}

func (s *Service) ApplyDNS(ctx context.Context, revision, actor string) (*protocol.Operation, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.operationCtx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeDaemonUnavailable, "daemon is shutting down")
	}
	if s.controlActive {
		return nil, protocol.NewError(protocol.CodeOperationInProgress, "another Headscale operation is running")
	}
	dns, err := s.DNS(ctx)
	if err != nil {
		return nil, err
	}
	if revision != dns.Revision {
		return nil, protocol.NewError(protocol.CodeConflict, "DNS settings changed; refresh before applying")
	}
	if !dns.Pending || !dns.Writable {
		return nil, protocol.NewError(protocol.CodeBadRequest, "no writable pending DNS changes")
	}
	if _, err := s.HeadscaleService(ctx); err != nil {
		return nil, err
	}
	op := s.ops.Start("dns.apply", s.control.Unit(), actor)
	s.controlActive = true
	s.operationWG.Add(1)
	go s.applyDNS(op.ID, revision)
	return op, nil
}

func (s *Service) applyDNS(id, revision string) {
	defer s.finishControlOperation()
	ctx, cancel := context.WithTimeout(s.operationCtx, 4*time.Minute)
	defer cancel()
	s.ops.SetState(id, protocol.OperationRunning, "writing DNS configuration")
	applied, err := s.dns.Apply(ctx, revision)
	if err != nil {
		s.ops.Fail(id, protocol.FromError(err), "DNS configuration was not applied")
		return
	}
	s.ops.AddStep(id, protocol.OperationStep{Name: "write DNS configuration", State: protocol.OperationSucceeded})
	if applied.RestartRequired {
		err = s.restartDNS(ctx, id)
	} else {
		err = s.hs.Health(ctx)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err == nil {
		if err := applied.Finish(finishCtx, true); err != nil {
			s.ops.Fail(id, protocol.FromError(err), "DNS applied but pending state could not be cleared")
			return
		}
		s.ops.SetState(id, protocol.OperationSucceeded, "DNS configuration applied")
		return
	}
	if restoreErr := applied.Finish(finishCtx, false); restoreErr != nil {
		s.ops.Fail(id, protocol.FromError(errors.Join(err, restoreErr)), "DNS application failed; configuration rollback failed")
		return
	}
	s.ops.AddStep(id, protocol.OperationStep{Name: "restore previous configuration", State: protocol.OperationSucceeded})
	var recoveryErr error
	if applied.RestartRequired {
		recoveryCtx := ctx
		if ctx.Err() != nil {
			var cancel context.CancelFunc
			recoveryCtx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
		}
		recoveryErr = s.restartDNS(recoveryCtx, id)
	}
	s.ops.Fail(id, protocol.FromError(errors.Join(err, recoveryErr)), "DNS application failed; previous configuration restored")
}

func (s *Service) restartDNS(ctx context.Context, id string) error {
	if _, err := s.control.Action(ctx, s.control.Unit(), protocol.ServiceRestart); err != nil {
		return err
	}
	state, err := s.control.State(ctx)
	if err != nil {
		return err
	}
	if state.ActiveState != "active" {
		return protocol.NewError(protocol.CodeServiceCtlUnavailable, "Headscale did not reach active state")
	}
	return s.waitForHeadscale(ctx, id)
}
