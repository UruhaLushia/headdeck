package accounts

import (
	"context"

	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"gorm.io/gorm"
)

func (s *Store) BindProvisionedHeadscale(ctx context.Context, id, userID, username string) error {
	return mutationError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r accountRecord
		if err := tx.Preload("Password").Where(&accountRecord{ID: id}).First(&r).Error; err != nil {
			return notFound(err)
		}
		if r.HeadscaleUserID != nil {
			if *r.HeadscaleUserID == userID {
				return nil
			}
			return ErrConflict
		}
		if r.Disabled || r.Role != string(policy.RoleMember) || r.Password == nil || r.Password.Username != username {
			return ErrConflict
		}
		return tx.Model(&r).Update("headscale_user_id", userID).Error
	}))
}

func (s *Store) SetRegistrationUsername(ctx context.Context, id, username string, version uint64) error {
	return mutationError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r accountRecord
		if err := tx.Preload("Security").Where(&accountRecord{ID: id}).First(&r).Error; err != nil {
			return notFound(err)
		}
		if r.Disabled || r.Role != string(policy.RoleMember) || r.HeadscaleUserID != nil || recordAccount(r).SecurityVersion != version {
			return ErrConflict
		}
		return setUsername(tx, id, username)
	}))
}
