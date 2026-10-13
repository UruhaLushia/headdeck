package accounts

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

func (s *Store) HasTOTP(ctx context.Context, id string) (bool, error) {
	secret, _, err := s.TOTP(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return len(secret) > 0, err
}
func (s *Store) RevokeSessions(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return bumpSecurity(tx, id) })
}
