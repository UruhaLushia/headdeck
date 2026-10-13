package accounts

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

var ErrNotFound = errors.New("account or binding not found")
var ErrDisabled = errors.New("account is disabled")

type Account struct {
	ID              string      `json:"id"`
	Label           string      `json:"label"`
	Username        string      `json:"username,omitempty"`
	Role            policy.Role `json:"role"`
	Disabled        bool        `json:"disabled"`
	HeadscaleUserID string      `json:"headscale_user_id,omitempty"`
	SecurityVersion uint64      `json:"-"`
	AvatarURL       string      `json:"-"`
}
type Identity struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}
type Binding struct {
	Identity
	AccountID string `json:"account_id"`
}
type Store struct{ db *gorm.DB }

func New(db *sql.DB) (*Store, error) {
	orm, err := openORM(db)
	if err != nil {
		return nil, err
	}
	return &Store{db: orm}, nil
}
func recordAccount(r accountRecord) *Account {
	a := &Account{ID: r.ID, Label: r.Label, Role: policy.Role(r.Role), Disabled: r.Disabled}
	if r.HeadscaleUserID != nil {
		a.HeadscaleUserID = *r.HeadscaleUserID
	}
	if r.Password != nil {
		a.Username = r.Password.Username
	}
	if r.Security != nil {
		a.SecurityVersion = r.Security.Revision
	}
	if r.Profile != nil {
		a.AvatarURL = r.Profile.AvatarURL
	}
	return a
}
func bumpSecurity(tx *gorm.DB, id string) error {
	var r securityRecord
	if err := tx.Where(&securityRecord{AccountID: id}).First(&r).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	r.AccountID = id
	r.Revision++
	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&r).Error; err != nil {
		return err
	}
	if tx.Migrator().HasTable(&database.SessionRecord{}) {
		return tx.Where(&database.SessionRecord{Owner: "account:" + id}).Delete(&database.SessionRecord{}).Error
	}
	return nil
}
func (s *Store) Save(ctx context.Context, a Account) (*Account, error) {
	role, err := policy.ParseRole(string(a.Role))
	if err != nil {
		return nil, err
	}
	a.Role = role
	a.Label = strings.TrimSpace(a.Label)
	if a.Label == "" || len(a.Label) > 256 {
		return nil, errors.New("account label must contain 1 to 256 bytes")
	}
	var user *string
	if a.HeadscaleUserID != "" {
		user = &a.HeadscaleUserID
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if a.ID == "" {
			a.ID = rand.Text()
			if err := tx.Create(&accountRecord{ID: a.ID, Label: a.Label, Role: string(a.Role), Disabled: a.Disabled, HeadscaleUserID: user}).Error; err != nil {
				return err
			}
			if a.Username != "" {
				return setUsername(tx, a.ID, a.Username)
			}
			return nil
		}
		var old accountRecord
		if err := tx.Where(&accountRecord{ID: a.ID}).First(&old).Error; err != nil {
			return notFound(err)
		}
		if old.Role == string(policy.RoleOwner) && !old.Disabled && (a.Role != policy.RoleOwner || a.Disabled) {
			var owners int64
			if err := tx.Model(&accountRecord{}).Where(&accountRecord{Role: string(policy.RoleOwner)}).Where("disabled = ?", false).Count(&owners).Error; err != nil {
				return err
			}
			if owners <= 1 {
				return errors.New("the last owner cannot be disabled or demoted")
			}
		}
		changed := old.Disabled != a.Disabled || old.HeadscaleUserID != nil && (user == nil || *old.HeadscaleUserID != *user)
		if err := tx.Model(&old).Updates(map[string]any{"label": a.Label, "role": a.Role, "disabled": a.Disabled, "headscale_user_id": user}).Error; err != nil {
			return err
		}
		if a.Username != "" {
			if err := setUsername(tx, a.ID, a.Username); err != nil {
				return err
			}
		}
		if changed {
			return bumpSecurity(tx, a.ID)
		}
		return nil
	})
	if err != nil {
		return nil, mutationError(err)
	}
	return s.Account(ctx, a.ID)
}
func (s *Store) Account(ctx context.Context, id string) (*Account, error) {
	var r accountRecord
	err := s.db.WithContext(ctx).Preload("Password").Preload("Security").Preload("Profile").Where(&accountRecord{ID: id}).First(&r).Error
	a := recordAccount(r)
	if err == nil && a.AvatarURL == "" {
		var bindings []bindingRecord
		err = s.db.WithContext(ctx).Where(&bindingRecord{AccountID: id, Issuer: "https://github.com"}).Order("subject").Find(&bindings).Error
		for _, binding := range bindings {
			if githubAvatar := githubAvatarURL(binding.Subject); githubAvatar != "" {
				a.AvatarURL = githubAvatar
				break
			}
		}
	}
	return a, notFound(err)
}
func (s *Store) List(ctx context.Context) ([]Account, error) {
	var rows []accountRecord
	err := s.db.WithContext(ctx).Preload("Password").Order("label").Order("id").Find(&rows).Error
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, *recordAccount(r))
	}
	return out, err
}
func (s *Store) HeadscaleLinked(ctx context.Context, id string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&accountRecord{}).Where(&accountRecord{HeadscaleUserID: &id}).Count(&n).Error
	return n > 0, err
}
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
