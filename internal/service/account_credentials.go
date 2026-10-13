package service

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) SetAccountPassword(ctx context.Context, id, username, password string) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	a, err := s.accounts.Account(ctx, id)
	if err != nil {
		return accountError(err)
	}
	if a.HeadscaleUserID != "" {
		username = a.Username
	}
	return accountError(s.accounts.SetPassword(ctx, id, username, password))
}
func (s *Service) AccountSecurity(ctx context.Context, id string) (*protocol.AccountSecurity, error) {
	a, err := s.accounts.Account(ctx, id)
	if err != nil {
		return nil, accountError(err)
	}
	out := &protocol.AccountSecurity{Account: *a}
	out.Identities, err = s.accounts.Bindings(ctx, id)
	if err != nil {
		return nil, accountError(err)
	}
	out.Passkeys, err = s.accounts.Passkeys(ctx, id)
	if err != nil {
		return nil, accountError(err)
	}
	out.Password, err = s.accounts.HasPassword(ctx, id)
	if err != nil {
		return nil, accountError(err)
	}
	out.TOTP, err = s.accounts.HasTOTP(ctx, id)
	return out, accountError(err)
}
func (s *Service) AccountSecurityAction(ctx context.Context, id string, v protocol.AccountSecurityAction) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if _, err := s.accounts.Account(ctx, id); err != nil {
		return accountError(err)
	}
	switch v.Action {
	case "totp.reset":
		return accountError(s.accounts.SetTOTP(ctx, id, nil, nil))
	case "passkey.delete":
		return accountError(s.accounts.DeletePasskey(ctx, id, v.ID))
	case "sessions.revoke":
		return accountError(s.accounts.RevokeSessions(ctx, id))
	default:
		return protocol.NewError(protocol.CodeBadRequest, "unknown security action")
	}
}
