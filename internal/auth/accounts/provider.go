package accounts

import "context"

type LoginRequest struct{ RedirectURI, State, Nonce, CodeChallenge string }
type Callback struct{ RedirectURI, Code, CodeVerifier, Nonce string }
type VerifiedIdentity struct {
	Identity
	Name           string
	Username       string
	AvatarURL      string
	VerifiedEmails []string
}

// Adapters return verified identities with stable provider-specific subjects.
type Provider interface {
	AuthorizationURL(context.Context, LoginRequest) (string, error)
	Exchange(context.Context, Callback) (VerifiedIdentity, error)
}
