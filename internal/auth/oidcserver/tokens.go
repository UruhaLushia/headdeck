package oidcserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"slices"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

func (s *storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	a, ok := request.(*authRequest)
	if !ok || !a.Done() {
		return "", time.Time{}, errInvalid
	}
	if err := s.validate(ctx, a.Profile); err != nil {
		return "", time.Time{}, err
	}
	id := rand.Text()
	expires := time.Now().Add(5 * time.Minute)
	payload, err := json.Marshal(a)
	if err != nil {
		return "", time.Time{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := prune(tx); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&tokenRecord{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 4096 {
			return errInvalid
		}
		return tx.Create(&tokenRecord{IDHash: digest(id), Issuer: s.issuer, Payload: payload, Expires: expires.Unix()}).Error
	})
	return id, expires, err
}
func (s *storage) token(ctx context.Context, id, subject string) (*authRequest, error) {
	var row tokenRecord
	if err := s.db.WithContext(ctx).Where("id_hash = ? AND issuer = ? AND expires > ?", digest(id), s.issuer, time.Now().Unix()).Take(&row).Error; err != nil {
		return nil, errInvalid
	}
	a, err := decodeRequest(row.Payload)
	if err != nil {
		return nil, err
	}
	if a.GetSubject() != subject {
		return nil, errInvalid
	}
	if err := s.validate(ctx, a.Profile); err != nil {
		return nil, err
	}
	return a, nil
}
func fillUserinfo(info *oidc.UserInfo, p Profile, scopes []string) {
	info.Subject = p.AccountID
	if slices.Contains(scopes, "profile") {
		info.Name = p.Name
		info.PreferredUsername = p.Username
		info.Picture = p.Picture
	}
	if slices.Contains(scopes, "email") && p.Email != "" {
		info.Email = p.Email
		info.EmailVerified = oidc.Bool(p.EmailVerified)
	}
}
func (s *storage) SetUserinfoFromRequest(ctx context.Context, info *oidc.UserInfo, request op.IDTokenRequest, scopes []string) error {
	a, ok := request.(*authRequest)
	if !ok {
		return errInvalid
	}
	if err := s.validate(ctx, a.Profile); err != nil {
		return err
	}
	fillUserinfo(info, a.Profile, scopes)
	return nil
}
func (s *storage) SetUserinfoFromToken(ctx context.Context, info *oidc.UserInfo, id, subject, _ string) error {
	a, err := s.token(ctx, id, subject)
	if err != nil {
		return err
	}
	fillUserinfo(info, a.Profile, a.GetScopes())
	return nil
}
func (s *storage) GetClientByClientID(_ context.Context, id string) (op.Client, error) {
	if id != s.client.ID {
		return nil, errInvalid
	}
	return s.client, nil
}
func (s *storage) AuthorizeClientIDSecret(_ context.Context, id, secret string) error {
	expected, actual := digest(s.secret), digest(secret)
	if id != s.client.ID || subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
		return errInvalid
	}
	return nil
}
func (s *storage) SigningKey(context.Context) (op.SigningKey, error) { return s.key, nil }
func (s *storage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}
func (s *storage) KeySet(context.Context) ([]op.Key, error) { return []op.Key{publicKey{s.key}}, nil }
func (s *storage) Health(ctx context.Context) error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}
