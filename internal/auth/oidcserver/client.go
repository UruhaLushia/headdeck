package oidcserver

import (
	"net/url"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

type client struct {
	ID, Redirect, Issuer string
	Lifetime             time.Duration
}

func (c client) GetID() string                       { return c.ID }
func (c client) RedirectURIs() []string              { return []string{c.Redirect} }
func (c client) PostLogoutRedirectURIs() []string    { return nil }
func (c client) ApplicationType() op.ApplicationType { return op.ApplicationTypeWeb }
func (c client) AuthMethod() oidc.AuthMethod         { return oidc.AuthMethodBasic }
func (c client) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}
func (c client) GrantTypes() []oidc.GrantType                                 { return []oidc.GrantType{oidc.GrantTypeCode} }
func (c client) LoginURL(id string) string                                    { return c.Issuer + "/login?id=" + url.QueryEscape(id) }
func (c client) AccessTokenType() op.AccessTokenType                          { return op.AccessTokenTypeBearer }
func (c client) IDTokenLifetime() time.Duration                               { return c.Lifetime }
func (c client) DevMode() bool                                                { return false }
func (c client) RestrictAdditionalIdTokenScopes() func([]string) []string     { return keepScopes }
func (c client) RestrictAdditionalAccessTokenScopes() func([]string) []string { return keepScopes }
func keepScopes(scopes []string) []string                                     { return scopes }
func (c client) IsScopeAllowed(string) bool                                   { return false }
func (c client) IDTokenUserinfoClaimsAssertion() bool                         { return true }
func (c client) ClockSkew() time.Duration                                     { return 0 }
