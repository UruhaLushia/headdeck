package service

import (
	"context"
	"errors"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) syncOIDCUsers(ctx context.Context, users []protocol.User) error {
	w := s.cfg.Server.Web
	if w.DefaultRole != "member" || !w.OIDCServer.Enabled {
		return nil
	}
	for _, u := range users {
		id, ok := strings.CutPrefix(u.ProviderID, w.OIDCIssuer()+"/")
		if !ok || u.Provider != "oidc" {
			continue
		}
		a, err := s.accounts.Account(ctx, id)
		if errors.Is(err, accounts.ErrNotFound) {
			continue
		}
		if err != nil {
			return accountError(err)
		}
		if a.HeadscaleUserID != "" || a.Disabled || a.Role != policy.RoleMember {
			continue
		}
		if err := s.accounts.BindProvisionedHeadscale(ctx, a.ID, u.ID, u.Name); err != nil {
			return accountError(err)
		}
		s.publish(protocol.EventAccountsChanged, a.ID)
	}
	return nil
}
