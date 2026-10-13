package accounts

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
)

func validateIdentity(i Identity) error {
	if strings.TrimSpace(i.Issuer) == "" || strings.TrimSpace(i.Subject) == "" || len(i.Issuer) > 2048 || len(i.Subject) > 1024 {
		return errors.New("identity requires issuer and subject within size limits")
	}
	return nil
}
func (s *Store) Bind(ctx context.Context, id string, i Identity) error {
	if err := validateIdentity(i); err != nil {
		return err
	}
	return mutationError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing bindingRecord
		err := tx.Where(&bindingRecord{Issuer: i.Issuer, Subject: i.Subject}).First(&existing).Error
		if err == nil {
			if existing.AccountID == id {
				return nil
			}
			return ErrConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err = tx.Model(&bindingRecord{}).Where(&bindingRecord{AccountID: id}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 64 {
			return errors.New("identity binding limit reached")
		}
		return tx.Create(&bindingRecord{Issuer: i.Issuer, Subject: i.Subject, AccountID: id}).Error
	}))
}
func (s *Store) Unbind(ctx context.Context, id string, i Identity) error {
	if err := validateIdentity(i); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where(&bindingRecord{AccountID: id, Issuer: i.Issuer, Subject: i.Subject}).Delete(&bindingRecord{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return bumpSecurity(tx, id)
	})
}
func (s *Store) Bindings(ctx context.Context, id string) ([]Binding, error) {
	var rows []bindingRecord
	err := s.db.WithContext(ctx).Where(&bindingRecord{AccountID: id}).Order("issuer").Order("subject").Find(&rows).Error
	out := make([]Binding, 0, len(rows))
	for _, r := range rows {
		out = append(out, Binding{Identity: Identity{Issuer: r.Issuer, Subject: r.Subject}, AccountID: r.AccountID})
	}
	return out, err
}
func (s *Store) Resolve(ctx context.Context, i Identity) (*Account, error) {
	if i.Issuer == LocalIssuer {
		return s.LocalAccount(ctx, i.Subject)
	}
	if err := validateIdentity(i); err != nil {
		return nil, err
	}
	var b bindingRecord
	if err := s.db.WithContext(ctx).Where(&bindingRecord{Issuer: i.Issuer, Subject: i.Subject}).First(&b).Error; err != nil {
		return nil, notFound(err)
	}
	return s.LocalAccount(ctx, b.AccountID)
}
