package oidcserver

import (
	"context"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"time"
)

func (s *storage) CreateAccessAndRefreshTokens(context.Context, op.TokenRequest, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, oidc.ErrUnsupportedGrantType()
}
func (s *storage) TokenRequestByRefreshToken(context.Context, string) (op.RefreshTokenRequest, error) {
	return nil, op.ErrInvalidRefreshToken
}
func (s *storage) GetRefreshTokenInfo(context.Context, string, string) (string, string, error) {
	return "", "", op.ErrInvalidRefreshToken
}
func (s *storage) TerminateSession(context.Context, string, string) error { return errInvalid }
func (s *storage) RevokeToken(ctx context.Context, id, subject, clientID string) *oidc.Error {
	a, err := s.token(ctx, id, subject)
	if err != nil {
		return nil
	}
	if a.GetClientID() != clientID {
		return oidc.ErrInvalidClient()
	}
	if err := s.db.WithContext(ctx).Where("id_hash = ? AND issuer = ?", digest(id), s.issuer).Delete(&tokenRecord{}).Error; err != nil {
		return oidc.ErrServerError()
	}
	return nil
}
func (s *storage) SetUserinfoFromScopes(context.Context, *oidc.UserInfo, string, string, []string) error {
	return nil
}
func (s *storage) SetIntrospectionFromToken(context.Context, *oidc.IntrospectionResponse, string, string, string) error {
	return errInvalid
}
func (s *storage) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}
func (s *storage) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, errInvalid
}
func (s *storage) ValidateJWTProfileScopes(context.Context, string, []string) ([]string, error) {
	return nil, oidc.ErrUnsupportedGrantType()
}
