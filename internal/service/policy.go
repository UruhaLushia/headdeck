package service

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"time"
)

func (s *Service) policyDatabase(ctx context.Context) (string, error) {
	if s.dns == nil {
		return "", protocol.NewError(protocol.CodeUnsupported, "policy management is unavailable")
	}
	mode, err := s.dns.PolicyMode()
	if err != nil || mode != "database" {
		return "", err
	}
	value, err := s.hs.Policy(ctx)
	if err != nil {
		return "", headscaleError(err)
	}
	return value, nil
}

func (s *Service) Policy(ctx context.Context) (*protocol.PolicySettings, error) {
	current, err := s.policyDatabase(ctx)
	if err != nil {
		return nil, err
	}
	return s.dns.Policy(ctx, current)
}

func (s *Service) CheckPolicy(ctx context.Context, value string) error {
	if len(value) == 0 || len(value) > 256<<10 {
		return protocol.NewError(protocol.CodeBadRequest, "policy must contain 1 to 256 KiB of HuJSON")
	}
	if err := s.hs.CheckPolicy(ctx, value); err != nil {
		return headscaleError(err)
	}
	return nil
}

func (s *Service) SavePolicy(ctx context.Context, req protocol.PolicyUpdate) (*protocol.PolicySettings, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return nil, err
	}
	if err := s.CheckPolicy(ctx, req.Policy); err != nil {
		return nil, err
	}
	current, err := s.policyDatabase(ctx)
	if err != nil {
		return nil, err
	}
	out, err := s.dns.SavePolicy(ctx, req, current)
	if err == nil {
		s.publish(protocol.EventPolicyChanged, "")
	}
	return out, err
}

func (s *Service) DiscardPolicy(ctx context.Context) error {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return err
	}
	if err := s.dns.DiscardPolicy(ctx); err != nil {
		return err
	}
	s.publish(protocol.EventPolicyChanged, "")
	return nil
}

func (s *Service) ApplyPolicy(ctx context.Context, revision, actor string) (*protocol.Operation, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if err := s.configurationWritable(); err != nil {
		return nil, err
	}
	out, err := s.Policy(ctx)
	if err != nil {
		return nil, err
	}
	if out.Revision != revision || !out.Pending {
		return nil, protocol.NewError(protocol.CodeConflict, "policy changed; refresh before applying")
	}
	if !out.Writable {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "policy is read-only")
	}
	if err := s.CheckPolicy(ctx, out.Policy); err != nil {
		return nil, err
	}
	if out.Mode == "file" {
		if _, err := s.HeadscaleService(ctx); err != nil {
			return nil, err
		}
	}
	target := "ACL policy"
	if s.control != nil {
		target = s.control.Unit()
	}
	op := s.ops.Start("policy.apply", target, actor)
	s.controlActive = true
	s.operationWG.Add(1)
	if out.Mode == "file" {
		go s.runConfiguration(op.ID, "ACL policy", func(ctx context.Context) (configApplication, error) {
			a, err := s.dns.ApplyPolicyFile(ctx, revision)
			if err != nil {
				return configApplication{}, err
			}
			return configApplication{restart: a.RestartRequired, finish: a.Finish}, nil
		})
	} else {
		go s.applyDatabasePolicy(op.ID, out.Policy)
	}
	return op, nil
}

func (s *Service) applyDatabasePolicy(id, value string) {
	defer s.finishControlOperation()
	ctx, cancel := context.WithTimeout(s.operationCtx, 30*time.Second)
	defer cancel()
	s.ops.SetState(id, protocol.OperationRunning, "applying ACL policy")
	if err := s.hs.SetPolicy(ctx, value); err != nil {
		s.ops.Fail(id, headscaleError(err), "policy application failed; verify the live policy before retrying")
		return
	}
	if err := s.dns.DiscardPolicy(ctx); err != nil {
		s.ops.Fail(id, protocol.FromError(err), "policy applied but draft could not be cleared")
		return
	}
	s.ops.SetState(id, protocol.OperationSucceeded, "ACL policy applied")
}
