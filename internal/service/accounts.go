package service

import (
	"context"
	"errors"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func accountError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, accounts.ErrNotFound) {
		return protocol.NewError(protocol.CodeNotFound, "account or identity binding not found")
	}
	if errors.Is(err, accounts.ErrConflict) {
		return protocol.NewError(protocol.CodeConflict, "account or identity is already linked")
	}
	return protocol.NewError(protocol.CodeBadRequest, "account operation rejected: %v", err)
}
func (s *Service) Accounts(ctx context.Context) ([]accounts.Account, error) {
	if s.accounts == nil {
		return nil, protocol.NewError(protocol.CodeInternal, "account store is unavailable")
	}
	a, err := s.accounts.List(ctx)
	return a, accountError(err)
}
func (s *Service) SaveAccount(ctx context.Context, id string, a accounts.Account) (*accounts.Account, error) {
	return s.SaveAccountCredentials(ctx, id, a, "")
}
func (s *Service) SaveAccountCredentials(ctx context.Context, id string, a accounts.Account, password string) (*accounts.Account, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if s.accounts == nil {
		return nil, protocol.NewError(protocol.CodeInternal, "account store is unavailable")
	}
	a.ID = id
	if id != "" && a.HeadscaleUserID == "" {
		a.Username = ""
	}
	if id == "" && a.HeadscaleUserID == "" && a.Username != "" && password == "" {
		return nil, protocol.NewError(protocol.CodeBadRequest, "password required for username login")
	}
	if a.HeadscaleUserID != "" {
		users, err := s.Users(ctx)
		if err != nil {
			return nil, err
		}
		found := false
		for _, u := range users {
			if u.ID == a.HeadscaleUserID {
				a.Username = u.Name
				found = true
				break
			}
		}
		if !found {
			return nil, protocol.NewError(protocol.CodeBadRequest, "Headscale user does not exist")
		}
	}
	var out *accounts.Account
	var err error
	if password != "" {
		out, err = s.accounts.SaveWithPassword(ctx, a, password)
	} else {
		out, err = s.accounts.Save(ctx, a)
	}
	if err == nil {
		s.publish(protocol.EventAccountsChanged, out.ID)
	}
	return out, accountError(err)
}
func (s *Service) IdentityBindings(ctx context.Context, id string) ([]accounts.Binding, error) {
	if s.accounts == nil {
		return nil, protocol.NewError(protocol.CodeInternal, "account store is unavailable")
	}
	if _, err := s.accounts.Account(ctx, id); err != nil {
		return nil, accountError(err)
	}
	out, err := s.accounts.Bindings(ctx, id)
	return out, accountError(err)
}
func (s *Service) BindIdentity(ctx context.Context, id string, i accounts.Identity, remove bool) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if s.accounts == nil {
		return protocol.NewError(protocol.CodeInternal, "account store is unavailable")
	}
	if _, err := s.accounts.Account(ctx, id); err != nil {
		return accountError(err)
	}
	if remove {
		account, err := s.accounts.Account(ctx, id)
		if err != nil {
			return accountError(err)
		}
		if account.Role == "owner" {
			return accountError(s.accounts.UnbindSelf(ctx, id, i))
		}
		return accountError(s.accounts.Unbind(ctx, id, i))
	}
	return accountError(s.accounts.Bind(ctx, id, i))
}
