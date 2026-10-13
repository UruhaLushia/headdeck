package service

import (
	"context"
	"errors"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"time"
)

type configApplication struct {
	restart bool
	finish  func(context.Context, bool) error
}

func (s *Service) runConfiguration(id, label string, apply func(context.Context) (configApplication, error)) {
	defer s.finishControlOperation()
	ctx, cancel := context.WithTimeout(s.operationCtx, 4*time.Minute)
	defer cancel()
	s.ops.SetState(id, protocol.OperationRunning, "writing "+label)
	a, err := apply(ctx)
	if err != nil {
		s.ops.Fail(id, protocol.FromError(err), label+" was not applied")
		return
	}
	s.ops.AddStep(id, protocol.OperationStep{Name: "write " + label, State: protocol.OperationSucceeded})
	if a.restart {
		err = s.restartDNS(ctx, id)
	} else {
		err = s.hs.Health(ctx)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err == nil {
		if err := a.finish(finishCtx, true); err != nil {
			s.ops.Fail(id, protocol.FromError(err), label+" applied but draft could not be cleared")
			return
		}
		s.ops.SetState(id, protocol.OperationSucceeded, label+" applied")
		return
	}
	if restoreErr := a.finish(finishCtx, false); restoreErr != nil {
		s.ops.Fail(id, protocol.FromError(errors.Join(err, restoreErr)), label+" failed; rollback failed")
		return
	}
	s.ops.AddStep(id, protocol.OperationStep{Name: "restore previous configuration", State: protocol.OperationSucceeded})
	var recoveryErr error
	if a.restart {
		recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 30*time.Second)
		recoveryErr = s.restartDNS(recoveryCtx, id)
		recoveryCancel()
	}
	s.ops.Fail(id, protocol.FromError(errors.Join(err, recoveryErr)), label+" failed; previous configuration restored")
}
