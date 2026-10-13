package accounts

import (
	"database/sql"
	"github.com/UruhaLushia/headdeck/internal/database"
	"gorm.io/gorm"
)

type accountRecord struct {
	ID              string          `gorm:"primaryKey"`
	Label           string          `gorm:"not null"`
	Role            string          `gorm:"not null"`
	Disabled        bool            `gorm:"not null"`
	HeadscaleUserID *string         `gorm:"uniqueIndex"`
	Password        *passwordRecord `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
	Security        *securityRecord `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
	Profile         *profileRecord  `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
}

func (accountRecord) TableName() string { return "accounts" }

type profileRecord struct {
	AccountID string `gorm:"primaryKey"`
	AvatarURL string `gorm:"not null"`
}

func (profileRecord) TableName() string { return "account_profiles" }

type passwordRecord struct {
	AccountID string `gorm:"primaryKey"`
	Username  string `gorm:"uniqueIndex;not null"`
	Hash      []byte
}

func (passwordRecord) TableName() string { return "account_passwords" }

type securityRecord struct {
	AccountID string `gorm:"primaryKey"`
	Revision  uint64 `gorm:"not null"`
}

func (securityRecord) TableName() string { return "account_security" }

type bindingRecord struct {
	Issuer    string        `gorm:"primaryKey"`
	Subject   string        `gorm:"primaryKey"`
	AccountID string        `gorm:"not null;index"`
	Account   accountRecord `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
}

func (bindingRecord) TableName() string { return "identity_bindings" }

type totpRecord struct {
	AccountID string        `gorm:"primaryKey"`
	Secret    []byte        `gorm:"not null"`
	LastStep  int64         `gorm:"not null"`
	Account   accountRecord `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
}

func (totpRecord) TableName() string { return "account_totp" }

type recoveryRecord struct {
	AccountID string        `gorm:"primaryKey"`
	Hash      string        `gorm:"primaryKey"`
	Account   accountRecord `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
}

func (recoveryRecord) TableName() string { return "account_recovery" }

type passkeyRecord struct {
	ID         string        `gorm:"primaryKey"`
	AccountID  string        `gorm:"not null;index"`
	Name       string        `gorm:"not null"`
	Credential []byte        `gorm:"not null"`
	Account    accountRecord `gorm:"foreignKey:AccountID;constraint:OnDelete:CASCADE"`
}

func (passkeyRecord) TableName() string { return "account_passkeys" }

func openORM(db *sql.DB) (*gorm.DB, error) {
	orm, err := database.Open(db)
	if err != nil {
		return nil, err
	}
	for _, model := range []any{&accountRecord{}, &bindingRecord{}, &passwordRecord{}, &securityRecord{}, &profileRecord{}, &totpRecord{}, &recoveryRecord{}, &passkeyRecord{}, &providerRecord{}} {
		if !orm.Migrator().HasTable(model) {
			if err = orm.Migrator().CreateTable(model); err != nil {
				return nil, err
			}
		}
	}
	return orm, nil
}
