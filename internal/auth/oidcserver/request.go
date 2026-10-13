package oidcserver

import (
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type Profile struct {
	AccountID, HeadscaleUserID     string
	Identity                       accounts.Identity
	Name, Username, Email, Picture string
	EmailVerified                  bool
	Version                        uint64
	AuthenticatedAt                time.Time
}

type authRequest struct {
	ID              string
	Request         oidc.AuthRequest
	Profile         Profile
	AuthenticatedAt time.Time
	RequestedAt     time.Time
}

func (a *authRequest) GetID() string                      { return a.ID }
func (a *authRequest) GetACR() string                     { return "" }
func (a *authRequest) GetAMR() []string                   { return nil }
func (a *authRequest) GetAudience() []string              { return []string{a.Request.ClientID} }
func (a *authRequest) GetAuthTime() time.Time             { return a.AuthenticatedAt }
func (a *authRequest) GetClientID() string                { return a.Request.ClientID }
func (a *authRequest) GetNonce() string                   { return a.Request.Nonce }
func (a *authRequest) GetRedirectURI() string             { return a.Request.RedirectURI }
func (a *authRequest) GetResponseType() oidc.ResponseType { return a.Request.ResponseType }
func (a *authRequest) GetResponseMode() oidc.ResponseMode { return a.Request.ResponseMode }
func (a *authRequest) GetScopes() []string                { return a.Request.Scopes }
func (a *authRequest) GetState() string                   { return a.Request.State }
func (a *authRequest) GetSubject() string                 { return a.Profile.AccountID }
func (a *authRequest) Done() bool                         { return a.Profile.AccountID != "" }
func (a *authRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.Request.CodeChallenge == "" {
		return nil
	}
	return &oidc.CodeChallenge{Challenge: a.Request.CodeChallenge, Method: a.Request.CodeChallengeMethod}
}
