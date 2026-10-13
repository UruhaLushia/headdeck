package accounts

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
)

const LocalIssuer = "urn:headdeck:local"

var ErrCredentials = errors.New("invalid username or password")
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.@+-]{0,254}$`)
var dummyHash = []byte("$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW")

func NormalizeUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !usernamePattern.MatchString(value) {
		return "", errors.New("invalid username")
	}
	return value, nil
}
func (s *Store) SetPassword(ctx context.Context, id, username, password string) error {
	username, err := NormalizeUsername(username)
	if err != nil {
		return err
	}
	if len(password) < 12 || len(password) > 72 {
		return errors.New("password must contain 12 to 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	return mutationError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account accountRecord
		if err := tx.Preload("Password").Where(&accountRecord{ID: id}).First(&account).Error; err != nil {
			return notFound(err)
		}
		if account.HeadscaleUserID != nil && account.Password != nil && account.Password.Username != username {
			return errors.New("username is managed by the linked Headscale user")
		}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&passwordRecord{AccountID: id, Username: username, Hash: hash}).Error; err != nil {
			return err
		}
		return bumpSecurity(tx, id)
	}))
}
func setUsername(tx *gorm.DB, id, username string) error {
	username, err := NormalizeUsername(username)
	if err != nil {
		return err
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_id"}}, DoUpdates: clause.AssignmentColumns([]string{"username"})}).Create(&passwordRecord{AccountID: id, Username: username}).Error
}
func (s *Store) SetUsername(ctx context.Context, id, username string) error {
	return mutationError(setUsername(s.db.WithContext(ctx), id, username))
}
func (s *Store) Authenticate(ctx context.Context, username, password string) (*Account, error) {
	normalized, err := NormalizeUsername(username)
	if err != nil || len(password) > 72 {
		return nil, ErrCredentials
	}
	var r passwordRecord
	err = s.db.WithContext(ctx).Where(&passwordRecord{Username: normalized}).First(&r).Error
	hash := r.Hash
	if err != nil || len(hash) == 0 {
		hash = dummyHash
	}
	valid := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if !valid || err != nil || len(r.Hash) == 0 {
		return nil, ErrCredentials
	}
	a, err := s.LocalAccount(ctx, r.AccountID)
	if err != nil {
		return nil, ErrCredentials
	}
	return a, nil
}
func (s *Store) HasPassword(ctx context.Context, id string) (bool, error) {
	var r passwordRecord
	err := s.db.WithContext(ctx).Where(&passwordRecord{AccountID: id}).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return len(r.Hash) > 0, err
}
func (s *Store) LocalAccount(ctx context.Context, id string) (*Account, error) {
	a, err := s.Account(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Disabled {
		return nil, ErrDisabled
	}
	return a, nil
}
func loginMethods(tx *gorm.DB, id string) (int, error) {
	var bindings, keys int64
	if err := tx.Model(&bindingRecord{}).Where(&bindingRecord{AccountID: id}).Count(&bindings).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&passkeyRecord{}).Where(&passkeyRecord{AccountID: id}).Count(&keys).Error; err != nil {
		return 0, err
	}
	var r passwordRecord
	err := tx.Where(&passwordRecord{AccountID: id}).First(&r).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if len(r.Hash) > 0 {
		bindings++
	}
	return int(bindings + keys), nil
}
func (s *Store) LoginMethods(ctx context.Context, id string) (int, error) {
	return loginMethods(s.db.WithContext(ctx), id)
}
