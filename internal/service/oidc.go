package service

import (
	"context"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/headscale"
	hi "github.com/UruhaLushia/headdeck/internal/headscale/identity"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) CheckOIDCTakeover(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.cfg.Server.Web.OIDCServer.Enabled || s.dns == nil {
		return protocol.NewError(protocol.CodeClientConfig, "Headscale login takeover is required")
	}
	if !s.oidcReady.Load() {
		return protocol.NewError(protocol.CodeDaemonUnavailable, "Headscale management is not ready; check daemon operations")
	}
	if err := hi.Pending(); err != nil {
		return err
	}
	return s.dns.CheckOIDC(s.cfg.Server.Web)
}

func (s *Service) OIDCUser(ctx context.Context, a *accounts.Account, issuer string) (*headscale.User, error) {
	users, err := s.hs.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	current, err := s.accounts.LocalAccount(ctx, a.ID)
	if err != nil {
		return nil, accountError(err)
	}
	if current.SecurityVersion != a.SecurityVersion {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "account security changed")
	}
	*a = *current
	if a.HeadscaleUserID == "" && (s.cfg.Server.Web.DefaultRole != "member" || a.Role != policy.RoleMember) {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "Headscale user binding is required")
	}
	for _, u := range users {
		if u.ID == a.HeadscaleUserID {
			if u.ProviderID != issuer+"/"+a.ID {
				return nil, protocol.NewError(protocol.CodePermissionDenied, "Headscale issuer binding must be switched first")
			}
			return &u, nil
		}
	}
	if a.HeadscaleUserID == "" {
		username := a.Username
		if len(username) < 2 || username[0] >= '0' && username[0] <= '9' || strings.Contains(username, "+") || strings.Count(username, "@") > 1 {
			username = "user-" + strings.ToLower(a.ID)
		}
		for _, u := range users {
			if u.ProviderID == issuer+"/"+a.ID && u.Provider == "oidc" {
				if err := s.accounts.BindProvisionedHeadscale(ctx, a.ID, u.ID, u.Name); err != nil {
					return nil, accountError(err)
				}
				a.HeadscaleUserID = u.ID
				s.publish(protocol.EventAccountsChanged, a.ID)
				return &u, nil
			}
		}
		for _, u := range users {
			if strings.EqualFold(u.Name, username) {
				suffix := "-" + strings.ToLower(a.ID)
				username = username[:min(len(username), 255-len(suffix))] + suffix
				break
			}
		}
		if username != a.Username {
			if err := s.accounts.SetRegistrationUsername(ctx, a.ID, username, a.SecurityVersion); err != nil {
				return nil, accountError(err)
			}
			a.Username = username
		}
		return &headscale.User{Name: a.Username, ProviderID: issuer + "/" + a.ID}, nil
	}
	return nil, protocol.NewError(protocol.CodePermissionDenied, "Headscale user is missing")
}
