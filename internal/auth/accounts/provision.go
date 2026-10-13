package accounts

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"unicode/utf8"
)

func (s *Store) Provision(ctx context.Context, v VerifiedIdentity, role policy.Role) (*Account, error) {
	if role != policy.RolePending && role != policy.RoleMember {
		return nil, errors.New("registration role must be pending or member")
	}
	if err := validateIdentity(v.Identity); err != nil {
		return nil, err
	}
	if a, err := s.Resolve(ctx, v.Identity); !errors.Is(err, ErrNotFound) {
		return a, err
	}
	label := strings.TrimSpace(v.Name)
	if label == "" {
		label = strings.TrimSpace(v.Username)
	}
	if label == "" {
		label = v.Subject
	}
	if len(label) > 256 {
		label = label[:256]
		for !utf8.ValidString(label) {
			label = label[:len(label)-1]
		}
	}
	a := &Account{ID: rand.Text(), Label: label, Role: role}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&accountRecord{ID: a.ID, Label: a.Label, Role: string(a.Role)}).Error; err != nil {
			return err
		}
		if err := tx.Create(&bindingRecord{Issuer: v.Issuer, Subject: v.Subject, AccountID: a.ID}).Error; err != nil {
			return err
		}
		username, err := NormalizeUsername(v.Username)
		if err != nil {
			username = "user-" + strings.ToLower(a.ID)
		}
		suffix := "-" + strings.ToLower(a.ID)
		for _, candidate := range []string{username, username[:min(len(username), 255-len(suffix))] + suffix} {
			result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoNothing: true}).Create(&passwordRecord{AccountID: a.ID, Username: candidate})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				a.Username = candidate
				return nil
			}
		}
		return ErrConflict
	})
	if err != nil {
		if errors.Is(mutationError(err), ErrConflict) {
			if existing, resolveErr := s.Resolve(ctx, v.Identity); !errors.Is(resolveErr, ErrNotFound) {
				return existing, resolveErr
			}
		}
		return nil, mutationError(err)
	}
	return a, nil
}
