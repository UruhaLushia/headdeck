package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	hi "github.com/UruhaLushia/headdeck/internal/headscale/identity"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) runOIDCTakeover(id string, recover bool) {
	defer s.finishControlOperation()
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	ctx, cancel := context.WithTimeout(s.operationCtx, 4*time.Minute)
	defer cancel()
	s.ops.SetState(id, protocol.OperationRunning, "checking Headscale identity bindings")
	if recover {
		err := s.recoverOIDCTakeover(ctx, id)
		if err != nil {
			s.ops.Fail(id, protocol.FromError(err), "login recovery failed")
			return
		}
		s.ops.SetState(id, protocol.OperationSucceeded, "login transaction recovered")
		return
	}
	if _, err := hi.LoadJournal(); err == nil {
		if err := s.recoverOIDCTakeover(ctx, id); err != nil {
			s.ops.Fail(id, protocol.FromError(err), "unfinished Headscale transaction could not be recovered")
			return
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		s.ops.Fail(id, protocol.FromError(err), "cannot read Headscale transaction")
		return
	}
	// A managed Headscale may have failed to start before its issuer was listening.
	if s.dns.CheckOIDC(s.cfg.Server.Web) == nil {
		if err := s.startOIDCService(ctx, id); err != nil {
			s.ops.Fail(id, protocol.FromError(err), "Headscale could not start")
			return
		}
	}
	j, err := s.prepareOIDCTakeover(ctx)
	if err != nil {
		s.ops.Fail(id, protocol.FromError(err), "login takeover was not started")
		return
	}
	unchanged := bytes.Equal(j.Configuration.Before, j.Configuration.After)
	for _, b := range j.Bindings {
		unchanged = unchanged && b.Before == b.After && b.Provider == "oidc"
	}
	if unchanged {
		s.oidcReady.Store(true)
		s.ops.SetState(id, protocol.OperationSucceeded, "Headscale login is already managed by Headdeck")
		return
	}
	if err := j.Save(true); err != nil {
		s.ops.Fail(id, protocol.FromError(err), "login transaction could not be saved")
		return
	}
	changed, err := s.applyOIDCTransaction(ctx, id, j)
	if err == nil {
		j.Committed = true
		if err := j.Save(false); err != nil {
			s.ops.Fail(id, protocol.FromError(err), "login applied; transaction requires recovery")
			return
		}
		if err := hi.ClearJournal(); err != nil {
			s.ops.Fail(id, protocol.FromError(err), "login applied; transaction cleanup failed")
			return
		}
		s.oidcReady.Store(true)
		s.ops.SetState(id, protocol.OperationSucceeded, "Headscale login is managed by Headdeck")
		return
	}
	recovery, stop := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stop()
	rollbackErr := s.restoreOIDCTransaction(recovery, id, j, changed)
	message := "login takeover failed; previous identities and configuration restored"
	if rollbackErr != nil {
		message = "login takeover failed; run hdkt headscale login recover"
	}
	s.ops.Fail(id, protocol.FromError(errors.Join(err, rollbackErr)), message)
}

func (s *Service) applyOIDCTransaction(ctx context.Context, id string, j *hi.Journal) (bool, error) {
	if err := s.stopOIDCService(ctx); err != nil {
		return false, err
	}
	changed, err := hi.Apply(ctx, j.UID, j.Bindings, false)
	if err != nil {
		return changed, err
	}
	s.ops.AddStep(id, protocol.OperationStep{Name: "update Headscale identities", State: protocol.OperationSucceeded})
	if err := s.dns.WriteOIDC(ctx, j.Configuration, false); err != nil {
		return changed, err
	}
	if err := s.startOIDCService(ctx, id); err != nil {
		return changed, err
	}
	if err := s.dns.CheckOIDC(s.cfg.Server.Web); err != nil {
		return changed, err
	}
	return changed, s.verifyOIDCBindings(ctx, j.Bindings)
}

func (s *Service) restoreOIDCTransaction(ctx context.Context, id string, j *hi.Journal, identities bool) error {
	if err := s.stopOIDCService(ctx); err != nil {
		return err
	}
	if identities {
		if _, err := hi.Apply(ctx, j.UID, j.Bindings, true); err != nil {
			return err
		}
	}
	if err := s.dns.WriteOIDC(ctx, j.Configuration, true); err != nil {
		return err
	}
	if err := s.startOIDCService(ctx, id); err != nil {
		return err
	}
	return hi.ClearJournal()
}

func (s *Service) recoverOIDCTakeover(ctx context.Context, id string) error {
	j, err := hi.LoadJournal()
	if err != nil {
		return err
	}
	if j.Unit != s.control.Unit() {
		return fmt.Errorf("login transaction belongs to another service unit")
	}
	if j.Committed {
		return hi.ClearJournal()
	}
	return s.restoreOIDCTransaction(ctx, id, j, true)
}
