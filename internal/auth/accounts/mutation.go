package accounts

import (
	"context"
	"gorm.io/gorm"
)

func (s *Store) SaveWithPassword(ctx context.Context, a Account, password string) (*Account, error) {
	var result *Account
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := &Store{db: tx}
		saved, err := store.Save(ctx, a)
		if err != nil {
			return err
		}
		if err = store.SetPassword(ctx, saved.ID, a.Username, password); err != nil {
			return err
		}
		result, err = store.Account(ctx, saved.ID)
		return err
	})
	return result, err
}
