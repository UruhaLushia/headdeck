package web

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"image/png"
	"net/http"
	"strings"
	"time"
)

func (s *Server) secretCipher() (cipher.AEAD, error) {
	sum := sha256.Sum256(s.securityKey)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Server) verifyTOTP(r *http.Request, id, code string) bool {
	encrypted, last, err := s.credentials.TOTP(r.Context(), id)
	if err != nil {
		return false
	}
	aead, err := s.secretCipher()
	if err != nil || len(encrypted) < aead.NonceSize() {
		return false
	}
	secret, err := aead.Open(nil, encrypted[:aead.NonceSize()], encrypted[aead.NonceSize():], []byte(id))
	if err != nil {
		return false
	}
	step := s.now().Unix() / 30
	for _, candidate := range []int64{step, step - 1, step + 1} {
		expected, e := totp.GenerateCode(string(secret), time.Unix(candidate*30, 0))
		if e == nil && candidate > last && expected == code {
			ok, e := s.credentials.ConsumeTOTP(r.Context(), id, candidate, "")
			return e == nil && ok
		}
	}
	ok, err := s.credentials.ConsumeTOTP(r.Context(), id, 0, strings.ToLower(strings.TrimSpace(code)))
	return err == nil && ok
}
func (s *Server) totpRoutes(mux *http.ServeMux) {
	s.selfEndpoint(mux, "POST /api/account/totp/start", func(_ http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		if _, _, err := s.credentials.TOTP(r.Context(), a.ID); !errors.Is(err, accounts.ErrNotFound) {
			return nil, protocol.NewError(protocol.CodeConflict, "2FA is already enabled")
		}
		name := a.Username
		if name == "" {
			name = a.Label
		}
		key, err := totp.Generate(totp.GenerateOpts{Issuer: "Headdeck", AccountName: name})
		if err != nil {
			return nil, err
		}
		id, ok := s.authStates.put(authState{kind: "totp-setup", account: a.ID, owner: a.ID, secret: key.Secret(), version: a.SecurityVersion}, s.now())
		if !ok {
			return nil, protocol.NewError(protocol.CodeRateLimited, "too many requests")
		}
		img, err := key.Image(240, 240)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err = png.Encode(&buf, img); err != nil {
			return nil, err
		}
		return map[string]string{"id": id, "secret": key.Secret(), "qr": "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())}, nil
	})
	s.selfEndpoint(mux, "POST /api/account/totp/finish", func(w http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		v, err := decodeRequest[struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		}](r)
		if err != nil {
			return nil, err
		}
		state, ok := s.authStates.take(v.ID, "totp-setup", a.ID, s.now())
		if !ok || state.version != a.SecurityVersion {
			return nil, protocol.NewError(protocol.CodeBadRequest, "setup expired")
		}
		valid, err := totp.ValidateCustom(v.Code, state.secret, s.now(), totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if err != nil || !valid {
			return nil, protocol.NewError(protocol.CodeBadRequest, "invalid verification code")
		}
		aead, err := s.secretCipher()
		if err != nil {
			return nil, err
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			return nil, err
		}
		secret := aead.Seal(nonce, nonce, []byte(state.secret), []byte(a.ID))
		codes := make([]string, 8)
		for i := range codes {
			var raw [12]byte
			if _, err = rand.Read(raw[:]); err != nil {
				return nil, err
			}
			codes[i] = hex.EncodeToString(raw[:])
		}
		if err = s.credentials.SetTOTP(r.Context(), a.ID, secret, codes); err != nil {
			return nil, err
		}
		if !s.verifyTOTP(r, a.ID, v.Code) {
			return nil, protocol.NewError(protocol.CodeInternal, "cannot activate 2FA")
		}
		return map[string]any{"recovery_codes": codes}, s.renewAccount(w, r, a.ID)
	})
	s.selfEndpoint(mux, "DELETE /api/account/totp", func(w http.ResponseWriter, r *http.Request, a *accounts.Account) (any, error) {
		if !s.limiter.allow("totp:"+a.ID, s.now()) {
			return nil, protocol.NewError(protocol.CodeRateLimited, "too many attempts")
		}
		v, err := decodeRequest[struct {
			Code string `json:"code"`
		}](r)
		if err != nil {
			return nil, err
		}
		if !s.verifyTOTP(r, a.ID, v.Code) {
			return nil, protocol.NewError(protocol.CodeBadRequest, "invalid verification code")
		}
		if err = s.credentials.SetTOTP(r.Context(), a.ID, nil, nil); err != nil {
			return nil, err
		}
		return map[string]bool{"updated": true}, s.renewAccount(w, r, a.ID)
	})
}
