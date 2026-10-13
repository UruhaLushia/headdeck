package service

import (
	"context"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func requireOwner(ctx context.Context, owner string) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.IsAdmin() {
		return nil
	}
	if p.Role != principal.RoleMember || p.HeadscaleUserID == "" || p.HeadscaleUserID != owner {
		return protocol.NewError(protocol.CodePermissionDenied, "resource does not belong to this account")
	}
	return nil
}

func (s *Service) requireNodeOwner(ctx context.Context, id string) error {
	if p, ok := principal.FromContext(ctx); !ok || p.IsAdmin() {
		return nil
	}
	node, err := s.Node(ctx, id)
	if err != nil {
		return err
	}
	return requireOwner(ctx, node.User.ID)
}

func (s *Service) requireKeyOwner(ctx context.Context, id string) error {
	if p, ok := principal.FromContext(ctx); !ok || p.IsAdmin() {
		return nil
	}
	keys, err := s.PreAuthKeys(ctx, "")
	if err != nil {
		return err
	}
	for _, key := range keys {
		if key.ID == id {
			return requireOwner(ctx, key.User.ID)
		}
	}
	return protocol.NewError(protocol.CodeNotFound, "key not found")
}
