package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	wap "github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
)

func (s *Server) handlePasskeyLoginStart(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow(s.clientIP(r), s.now()) {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many attempts"))
		return
	}
	config, err := s.passkeyConfig()
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeBadRequest, "%v", err))
		return
	}
	options, session, err := config.BeginDiscoverableLogin(wa.WithUserVerification(wap.VerificationRequired))
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	id, ok := s.authStates.put(authState{kind: "passkey-login", ceremony: session}, s.now())
	if !ok {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many attempts"))
		return
	}
	s.challengeCookie(w, id, 300)
	s.writeJSON(w, http.StatusOK, options.Response)
}

func (s *Server) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow(s.clientIP(r), s.now()) {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many attempts"))
		return
	}
	cookie, err := r.Cookie("headdeck_auth")
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "sign in again"))
		return
	}
	state, ok := s.authStates.take(cookie.Value, "passkey-login", "", s.now())
	if !ok {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "sign in again"))
		return
	}
	config, err := s.passkeyConfig()
	if err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	user, credential, err := config.FinishPasskeyLogin(func(rawID, handle []byte) (wa.User, error) {
		return s.passkeyLoginUser(r.Context(), rawID, handle)
	}, *state.ceremony, r)
	if err != nil || credential.Authenticator.CloneWarning {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "passkey verification failed"))
		return
	}
	u := user.(*passkeyUser)
	key := accounts.Passkey{ID: base64.RawURLEncoding.EncodeToString(credential.ID), Credential: *credential}
	if err = s.credentials.SavePasskey(r.Context(), u.account.ID, key, true); err != nil {
		s.writeError(w, protocol.FromError(err))
		return
	}
	s.completeLogin(w, r, u.account, accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: accounts.LocalIssuer, Subject: u.account.ID}}, true, false)
}

func (s *Server) passkeyLoginUser(ctx context.Context, rawID, handle []byte) (wa.User, error) {
	a, err := s.credentials.LocalAccount(ctx, string(handle))
	if err != nil {
		return nil, err
	}
	u, err := s.passkeyUser(ctx, a)
	if err != nil {
		return nil, err
	}
	for _, c := range u.credentials {
		if bytes.Equal(c.ID, rawID) {
			return u, nil
		}
	}
	return nil, accounts.ErrNotFound
}
