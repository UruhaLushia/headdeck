package accounts

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

func (s *Store) UnbindSelf(ctx context.Context, id string, i Identity) error {
	if err := validateIdentity(i); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		n, err := loginMethods(tx, id)
		if err != nil {
			return err
		}
		if n <= 1 {
			return errors.New("cannot remove the last sign-in method")
		}
		r := tx.Where(&bindingRecord{AccountID: id, Issuer: i.Issuer, Subject: i.Subject}).Delete(&bindingRecord{})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return ErrNotFound
		}
		return bumpSecurity(tx, id)
	})
}
