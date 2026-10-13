package web

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	wap "github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
)

type passkeyUser struct {
	account     *accounts.Account
	credentials []wa.Credential
}

func (u *passkeyUser) WebAuthnID() []byte { return []byte(u.account.ID) }
func (u *passkeyUser) WebAuthnName() string {
	if u.account.Username != "" {
		return u.account.Username
	}
	return u.account.ID
}
func (u *passkeyUser) WebAuthnDisplayName() string          { return u.account.Label }
func (u *passkeyUser) WebAuthnCredentials() []wa.Credential { return u.credentials }
func (s *Server) passkeyConfig() (*wa.WebAuthn, error) {
	public := s.cfg.PublicURL
	if public == "" {
		public = "http://" + s.cfg.Listen
	}
	u, err := url.Parse(public)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return nil, errors.New("passkeys require HTTPS")
	}
	return wa.New(&wa.Config{RPID: u.Hostname(), RPDisplayName: "Headdeck", RPOrigins: []string{u.Scheme + "://" + u.Host}, AuthenticatorSelection: wap.AuthenticatorSelection{UserVerification: wap.VerificationRequired}})
}
func (s *Server) passkeyUser(ctx context.Context, a *accounts.Account) (*passkeyUser, error) {
	keys, err := s.credentials.Passkeys(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	u := &passkeyUser{account: a}
	for _, key := range keys {
		u.credentials = append(u.credentials, key.Credential)
	}
	return u, nil
}
func (s *Server) passkeyRoutes(mux *http.ServeMux) {
	s.selfEndpoint(mux, "POST /api/account/passkeys/start", func(_ http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		config, err := s.passkeyConfig()
		if err != nil {
			return nil, err
		}
		user, err := s.passkeyUser(r.Context(), a)
		if err != nil {
			return nil, err
		}
		options, session, err := config.BeginRegistration(user, wa.WithResidentKeyRequirement(wap.ResidentKeyRequirementRequired), wa.WithExclusions(wa.Credentials(user.credentials).CredentialDescriptors()))
		if err != nil {
			return nil, err
		}
		id, ok := s.authStates.put(authState{kind: "passkey-register", account: a.ID, owner: a.ID, version: a.SecurityVersion, ceremony: session}, s.now())
		if !ok {
			return nil, protocol.NewError(protocol.CodeRateLimited, "too many requests")
		}
		return map[string]any{"id": id, "options": options.Response}, nil
	})
	s.selfEndpoint(mux, "POST /api/account/passkeys/finish/{id}", func(_ http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		state, ok := s.authStates.take(r.PathValue("id"), "passkey-register", a.ID, s.now())
		if !ok || state.version != a.SecurityVersion {
			return nil, protocol.NewError(protocol.CodeBadRequest, "setup expired")
		}
		config, err := s.passkeyConfig()
		if err != nil {
			return nil, err
		}
		user, err := s.passkeyUser(r.Context(), a)
		if err != nil {
			return nil, err
		}
		r.Body = http.MaxBytesReader(nil, r.Body, 64<<10)
		credential, err := config.FinishRegistration(user, *state.ceremony, r)
		if err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "passkey verification failed")
		}
		key := accounts.Passkey{ID: base64.RawURLEncoding.EncodeToString(credential.ID), Name: "Passkey", Credential: *credential}
		return key, s.credentials.SavePasskey(r.Context(), a.ID, key, false)
	})
	s.selfEndpoint(mux, "DELETE /api/account/passkeys/{id}", func(w http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		if err := s.credentials.DeletePasskey(r.Context(), a.ID, r.PathValue("id")); err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest, "%v", err)
		}
		return map[string]bool{"updated": true}, s.renewAccount(w, r, a.ID)
	})
	mux.HandleFunc("POST /api/auth/passkey/start", s.handlePasskeyLoginStart)
	mux.HandleFunc("POST /api/auth/passkey/finish", s.handlePasskeyLoginFinish)
}
