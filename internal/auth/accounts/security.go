package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Passkey struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Credential webauthn.Credential `json:"-"`
}

func (s *Store) Passkeys(ctx context.Context, id string) ([]Passkey, error) {
	var rows []passkeyRecord
	err := s.db.WithContext(ctx).Where(&passkeyRecord{AccountID: id}).Order("id").Find(&rows).Error
	out := make([]Passkey, 0, len(rows))
	for _, r := range rows {
		var c webauthn.Credential
		if e := json.Unmarshal(r.Credential, &c); e != nil {
			return nil, e
		}
		out = append(out, Passkey{ID: r.ID, Name: passkeyName(r.Name, c), Credential: c})
	}
	return out, err
}
func (s *Store) SavePasskey(ctx context.Context, id string, p Passkey, update bool) error {
	data, err := json.Marshal(p.Credential)
	if err != nil {
		return err
	}
	return mutationError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if update {
			var old passkeyRecord
			if err := tx.Where(&passkeyRecord{AccountID: id, ID: p.ID}).First(&old).Error; err != nil {
				return notFound(err)
			}
			var previous webauthn.Credential
			if err := json.Unmarshal(old.Credential, &previous); err != nil {
				return err
			}
			if p.Credential.Authenticator.SignCount < previous.Authenticator.SignCount {
				return errors.New("passkey counter moved backwards")
			}
			r := tx.Model(&passkeyRecord{}).Where(&passkeyRecord{AccountID: id, ID: p.ID}).Update("credential", data)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected == 0 {
				return ErrNotFound
			}
			return nil
		}
		var n int64
		if err := tx.Model(&passkeyRecord{}).Where(&passkeyRecord{AccountID: id}).Count(&n).Error; err != nil {
			return err
		}
		if n >= 10 {
			return errors.New("passkey limit reached")
		}
		return tx.Create(&passkeyRecord{ID: p.ID, AccountID: id, Name: passkeyName(p.Name, p.Credential), Credential: data}).Error
	}))
}
func (s *Store) DeletePasskey(ctx context.Context, id, key string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		n, err := loginMethods(tx, id)
		if err != nil {
			return err
		}
		if n <= 1 {
			return errors.New("cannot remove the last sign-in method")
		}
		r := tx.Where(&passkeyRecord{AccountID: id, ID: key}).Delete(&passkeyRecord{})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return ErrNotFound
		}
		return bumpSecurity(tx, id)
	})
}
func (s *Store) TOTP(ctx context.Context, id string) ([]byte, int64, error) {
	var r totpRecord
	err := s.db.WithContext(ctx).Where(&totpRecord{AccountID: id}).First(&r).Error
	return r.Secret, r.LastStep, notFound(err)
}
func (s *Store) SetTOTP(ctx context.Context, id string, secret []byte, codes []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&recoveryRecord{AccountID: id}).Delete(&recoveryRecord{}).Error; err != nil {
			return err
		}
		if len(secret) == 0 {
			if err := tx.Where(&totpRecord{AccountID: id}).Delete(&totpRecord{}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&totpRecord{AccountID: id, Secret: secret}).Error; err != nil {
				return err
			}
		}
		for _, code := range codes {
			sum := sha256.Sum256([]byte(code))
			if err := tx.Create(&recoveryRecord{AccountID: id, Hash: hex.EncodeToString(sum[:])}).Error; err != nil {
				return err
			}
		}
		return bumpSecurity(tx, id)
	})
}
func (s *Store) ConsumeTOTP(ctx context.Context, id string, step int64, code string) (bool, error) {
	var r *gorm.DB
	if step > 0 {
		r = s.db.WithContext(ctx).Model(&totpRecord{}).Where(&totpRecord{AccountID: id}).Where(clause.Lt{Column: "last_step", Value: step}).Update("last_step", step)
	} else {
		sum := sha256.Sum256([]byte(code))
		r = s.db.WithContext(ctx).Where(&recoveryRecord{AccountID: id, Hash: hex.EncodeToString(sum[:])}).Delete(&recoveryRecord{})
	}
	return r.RowsAffected == 1, r.Error
}
