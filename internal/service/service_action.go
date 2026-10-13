package service

import (
	"context"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

const (
	serviceHealthTimeout  = 90 * time.Second
	serviceHealthInterval = 2 * time.Second
)

func (s *Service) StartHeadscaleAction(ctx context.Context, action protocol.ServiceAction, requestedBy string) (*protocol.Operation, error) {
	if !action.Valid() {
		return nil, protocol.NewError(protocol.CodeBadRequest, "unknown service action")
	}
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.operationCtx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeDaemonUnavailable, "daemon is shutting down")
	}
	if s.controlActive {
		return nil, protocol.NewError(protocol.CodeOperationInProgress, "another Headscale service operation is running")
	}
	state, err := s.HeadscaleService(ctx)
	if err != nil {
		return nil, err
	}
	if action == protocol.ServiceReload && !state.State.CanReload {
		return nil, protocol.NewError(protocol.CodeUnsupported, "this unit does not support reload; use restart")
	}
	unit := s.control.Unit()
	op := s.ops.Start("headscale."+string(action), unit, requestedBy)
	opID := op.ID
	s.controlActive = true
	s.operationWG.Add(1)
	s.ops.SetState(opID, protocol.OperationRunning, "submitting "+string(action))
	s.ops.AddStep(opID, protocol.OperationStep{
		Name:  "request accepted",
		State: protocol.OperationSucceeded,
	})

	go s.runServiceAction(opID, unit, action)
	return op, nil
}

func (s *Service) finishControlOperation() {
	s.controlMu.Lock()
	s.controlActive = false
	s.controlMu.Unlock()
	s.operationWG.Done()
}

func (s *Service) runServiceAction(opID, unit string, action protocol.ServiceAction) {
	defer s.finishControlOperation()
	ctx, cancel := context.WithTimeout(s.operationCtx, 2*time.Minute)
	defer cancel()
	job, err := s.control.Action(ctx, unit, action)
	if err != nil {
		s.ops.AddStep(opID, protocol.OperationStep{
			Name:    "systemd " + string(action),
			State:   protocol.OperationFailed,
			Message: err.Error(),
		})
		s.ops.Fail(opID, protocol.NewError(protocol.CodeServiceCtlUnavailable, "%s", err), "systemd action failed")
		return
	}
	s.ops.AddStep(opID, protocol.OperationStep{
		Name:    "systemd " + string(action),
		State:   protocol.OperationSucceeded,
		Message: "job " + job,
	})

	state, err := s.control.State(ctx)
	expected := "active"
	if action == protocol.ServiceStop {
		expected = "inactive"
	}
	if err != nil || state.ActiveState != expected {
		s.ops.Fail(opID, protocol.NewError(protocol.CodeServiceCtlUnavailable, "service did not reach %s state", expected), "unexpected service state")
		return
	}
	if action == protocol.ServiceStop {
		s.mu.Lock()
		s.snapshot = nil
		s.mu.Unlock()
		s.ops.SetState(opID, protocol.OperationSucceeded, "Headscale service stopped")
		return
	}
	if err := s.waitForHeadscale(ctx, opID); err != nil {
		s.ops.Fail(opID, protocol.FromError(err), "Headscale did not recover")
		return
	}
	s.ops.SetState(opID, protocol.OperationSucceeded, "Headscale API is answering again")
}

func (s *Service) waitForHeadscale(ctx context.Context, opID string) error {
	ctx, cancel := context.WithTimeout(ctx, serviceHealthTimeout)
	defer cancel()
	deadline := time.Now().Add(serviceHealthTimeout)
	stepAdded := false
	for {
		if time.Now().After(deadline) {
			err := protocol.NewError(protocol.CodeRequestTimeout,
				"Headscale API did not answer within %s", serviceHealthTimeout)
			s.ops.AddStep(opID, protocol.OperationStep{
				Name:    "headscale api health",
				State:   protocol.OperationTimeout,
				Message: err.Message,
				Error:   err,
			})
			return err
		}
		if err := s.hs.Health(ctx); err == nil {
			s.ops.AddStep(opID, protocol.OperationStep{
				Name:  "headscale api health",
				State: protocol.OperationSucceeded,
			})
			s.Probe(ctx, true)
			return nil
		} else if !stepAdded {
			stepAdded = true
			s.ops.AddStep(opID, protocol.OperationStep{
				Name:    "headscale api health",
				State:   protocol.OperationRunning,
				Message: "waiting for the API to answer",
			})
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(serviceHealthInterval):
		}
	}
}
