package service

import (
	"context"
	"errors"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) LoginAccount(ctx context.Context, identity accounts.VerifiedIdentity, register bool) (*accounts.Account, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if s.accounts == nil {
		return nil, protocol.NewError(protocol.CodeUnauthenticated, "account login is unavailable")
	}
	a, err := s.accounts.Resolve(ctx, identity.Identity)
	if errors.Is(err, accounts.ErrNotFound) && register {
		a, err = s.accounts.Provision(ctx, identity, policy.Role(s.cfg.Server.Web.DefaultRole))
		if err == nil {
			s.publish(protocol.EventAccountsChanged, a.ID)
		}
	}
	if err == nil && identity.AvatarURL != "" {
		err = s.accounts.SetAvatar(ctx, a.ID, identity.AvatarURL)
	}
	return a, err
}
