package accounts

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

func (s *Store) CheckHeadscaleUsername(ctx context.Context, userID, name string) error {
	var account accountRecord
	err := s.db.WithContext(ctx).Where(&accountRecord{HeadscaleUserID: &userID}).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	username, err := NormalizeUsername(name)
	if err != nil {
		return err
	}
	var password passwordRecord
	err = s.db.WithContext(ctx).Where(&passwordRecord{Username: username}).First(&password).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if password.AccountID != account.ID {
		return ErrConflict
	}
	return nil
}

func (s *Store) SyncHeadscaleUsers(ctx context.Context, names map[string]string) error {
	return mutationError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []accountRecord
		if err := tx.Preload("Password").Where("headscale_user_id IS NOT NULL").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			name, exists := names[*r.HeadscaleUserID]
			if !exists {
				if err := tx.Model(&r).Update("headscale_user_id", nil).Error; err != nil {
					return err
				}
				if err := bumpSecurity(tx, r.ID); err != nil {
					return err
				}
				continue
			}
			username, err := NormalizeUsername(name)
			if err != nil {
				return err
			}
			if r.Password != nil && r.Password.Username == username {
				continue
			}
			if err := setUsername(tx, r.ID, username); err != nil {
				return err
			}
		}
		return nil
	}))
}
