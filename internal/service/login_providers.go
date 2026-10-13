package service

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) SetLoginProviderReload(reload func(context.Context) error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.loginProviderReload = reload
}
func (s *Service) LoginProviders(ctx context.Context) ([]accounts.LoginProvider, error) {
	out, err := s.accounts.Providers(ctx)
	for i := range out {
		out[i].SecretConfigured = out[i].ClientSecret != ""
		out[i].ClientSecret = ""
	}
	return out, accountError(err)
}
func (s *Service) SaveLoginProvider(ctx context.Context, p accounts.LoginProvider) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	defs, err := s.accounts.Providers(ctx)
	if err != nil {
		return accountError(err)
	}
	exists := false
	for _, old := range defs {
		if old.ID == p.ID {
			exists = true
			if p.ClientSecret == "" {
				p.ClientSecret = old.ClientSecret
			}
			if old.Kind != p.Kind || old.Issuer != p.Issuer {
				return protocol.NewError(protocol.CodeBadRequest, "create a new provider when changing identity issuer")
			}
			if p.Kind == "oauth" && (old.AuthURL != p.AuthURL || old.TokenURL != p.TokenURL || old.UserinfoURL != p.UserinfoURL || old.SubjectClaim != p.SubjectClaim) {
				return protocol.NewError(protocol.CodeBadRequest, "create a new provider when changing OAuth identity endpoints or subject")
			}
		}
	}
	if !exists && len(defs) >= 16 {
		return protocol.NewError(protocol.CodeBadRequest, "provider limit reached")
	}
	if p.Enabled && s.Config().Server.Web.PublicURL == "" {
		return protocol.NewError(protocol.CodeBadRequest, "public URL required")
	}
	if err := s.accounts.SaveProvider(ctx, p); err != nil {
		return accountError(err)
	}
	s.publish(protocol.EventProvidersChanged, p.ID)
	if s.loginProviderReload != nil {
		return s.loginProviderReload(ctx)
	}
	return nil
}
func (s *Service) DeleteLoginProvider(ctx context.Context, id string) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accounts.DeleteProvider(ctx, id); err != nil {
		return accountError(err)
	}
	s.publish(protocol.EventProvidersChanged, id)
	if s.loginProviderReload != nil {
		return s.loginProviderReload(ctx)
	}
	return nil
}
