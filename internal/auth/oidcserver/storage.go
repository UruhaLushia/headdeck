package oidcserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

type requestRecord struct {
	ID        string `gorm:"primaryKey"`
	Issuer    string
	AccountID string
	CodeHash  *string `gorm:"uniqueIndex"`
	Payload   []byte
	Expires   int64 `gorm:"index"`
}
type tokenRecord struct {
	IDHash  string `gorm:"primaryKey"`
	Issuer  string
	Payload []byte
	Expires int64 `gorm:"index"`
}

func (requestRecord) TableName() string { return "oidc_requests" }
func (tokenRecord) TableName() string   { return "oidc_tokens" }

type storage struct {
	db             *gorm.DB
	key            signingKey
	issuer, secret string
	client         client
	validate       func(context.Context, Profile) error
}

var errInvalid = errors.New("invalid or expired OIDC request")

func digest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func (s *storage) CreateAuthRequest(ctx context.Context, req *oidc.AuthRequest, _ string) (op.AuthRequest, error) {
	if req.ResponseType != oidc.ResponseTypeCode {
		return nil, oidc.ErrInvalidRequest()
	}
	for _, prompt := range req.Prompt {
		if prompt == "none" {
			return nil, oidc.ErrLoginRequired()
		}
	}
	a := &authRequest{ID: rand.Text(), Request: *req, RequestedAt: time.Now().UTC()}
	payload, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := prune(tx); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&requestRecord{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 1024 {
			return errors.New("too many OIDC requests")
		}
		return tx.Create(&requestRecord{ID: a.ID, Issuer: s.issuer, Payload: payload, Expires: time.Now().Add(5 * time.Minute).Unix()}).Error
	})
	return a, err
}
func prune(tx *gorm.DB) error {
	for _, model := range []any{&requestRecord{}, &tokenRecord{}} {
		if err := tx.Where("expires <= ?", time.Now().Unix()).Delete(model).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	var row requestRecord
	if err := s.db.WithContext(ctx).Where("id = ? AND issuer = ? AND expires > ?", id, s.issuer, time.Now().Unix()).Take(&row).Error; err != nil {
		return nil, errInvalid
	}
	return decodeRequest(row.Payload)
}
func decodeRequest(payload []byte) (*authRequest, error) {
	var a authRequest
	if err := json.Unmarshal(payload, &a); err != nil {
		return nil, err
	}
	return &a, nil
}
func (s *storage) authenticate(ctx context.Context, id string, p Profile) error {
	if err := s.validate(ctx, p); err != nil {
		return err
	}
	request, err := s.AuthRequestByID(ctx, id)
	if err != nil {
		return err
	}
	a := request.(*authRequest)
	if a.Done() {
		return errInvalid
	}
	a.Profile = p
	a.AuthenticatedAt = p.AuthenticatedAt
	if a.AuthenticatedAt.IsZero() {
		return errInvalid
	}
	payload, err := json.Marshal(a)
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Model(&requestRecord{}).Where("id = ? AND issuer = ? AND account_id = ? AND expires > ?", id, s.issuer, "", time.Now().Unix()).Updates(map[string]any{"payload": payload, "account_id": p.AccountID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errInvalid
	}
	return nil
}
func (s *storage) SaveAuthCode(ctx context.Context, id, code string) error {
	request, err := s.AuthRequestByID(ctx, id)
	if err != nil {
		return err
	}
	a := request.(*authRequest)
	if !a.Done() {
		return errInvalid
	}
	if err := s.validate(ctx, a.Profile); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Model(&requestRecord{}).Where("id = ? AND issuer = ? AND code_hash IS NULL AND expires > ?", id, s.issuer, time.Now().Unix()).Updates(map[string]any{"code_hash": digest(code), "expires": time.Now().Add(time.Minute).Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errInvalid
	}
	return nil
}
func (s *storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	var a *authRequest
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row requestRecord
		if err := tx.Where("code_hash = ? AND issuer = ? AND expires > ?", digest(code), s.issuer, time.Now().Unix()).Take(&row).Error; err != nil {
			return errInvalid
		}
		// Consume before exchange so concurrent requests cannot replay the code.
		result := tx.Model(&requestRecord{}).Where("id = ? AND code_hash = ?", row.ID, digest(code)).Update("code_hash", digest(rand.Text()))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInvalid
		}
		var err error
		a, err = decodeRequest(row.Payload)
		return err
	})
	if err != nil {
		return nil, err
	}
	if !a.Done() {
		return nil, errInvalid
	}
	if err := s.validate(ctx, a.Profile); err != nil {
		return nil, err
	}
	return a, nil
}
func (s *storage) DeleteAuthRequest(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ? AND issuer = ?", id, s.issuer).Delete(&requestRecord{}).Error
}
