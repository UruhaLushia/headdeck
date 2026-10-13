package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm/clause"
	"net/url"
	"regexp"
	"strings"
)

type LoginProvider struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	Enabled          bool     `json:"enabled"`
	AutoRegister     bool     `json:"auto_register"`
	Issuer           string   `json:"issuer"`
	ClientID         string   `json:"client_id"`
	ClientSecret     string   `json:"client_secret,omitempty"`
	AuthURL          string   `json:"auth_url,omitempty"`
	TokenURL         string   `json:"token_url,omitempty"`
	UserinfoURL      string   `json:"userinfo_url,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	SubjectClaim     string   `json:"subject_claim,omitempty"`
	NameClaim        string   `json:"name_claim,omitempty"`
	AvatarClaim      string   `json:"avatar_claim,omitempty"`
	SecretConfigured bool     `json:"secret_configured,omitempty"`
}
type providerRecord struct {
	ID     string `gorm:"primaryKey"`
	Config []byte `gorm:"not null"`
}

func (providerRecord) TableName() string { return "login_providers" }
func (p LoginProvider) Validate() error {
	if !providerIDPattern.MatchString(p.ID) || p.ID == "oidc" || p.ID == "github" || len(p.Name) == 0 || len(p.Name) > 80 {
		return errors.New("invalid provider ID or name")
	}
	if p.Kind != "oidc" && p.Kind != "oauth" && p.Kind != "github" {
		return errors.New("unknown provider type")
	}
	if len(p.ClientID) > 2048 || len(p.ClientSecret) > 8192 || len(p.Scopes) > 20 {
		return errors.New("provider settings exceed size limits")
	}
	if p.Enabled && p.ClientID == "" {
		return errors.New("client ID required")
	}
	urls := []string{}
	if p.Kind == "oidc" {
		urls = append(urls, p.Issuer)
	}
	if p.Kind == "oauth" {
		urls = append(urls, p.AuthURL, p.TokenURL, p.UserinfoURL)
	}
	for _, value := range urls {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return errors.New("provider URLs must use HTTPS without credentials")
		}
	}
	for _, claim := range []string{p.SubjectClaim, p.NameClaim, p.AvatarClaim} {
		if len(claim) > 128 || strings.Contains(claim, ".") {
			return errors.New("invalid identity claim")
		}
	}
	return nil
}

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

func (s *Store) Providers(ctx context.Context) ([]LoginProvider, error) {
	var rows []providerRecord
	err := s.db.WithContext(ctx).Order("id").Find(&rows).Error
	out := make([]LoginProvider, 0, len(rows))
	for _, r := range rows {
		var p LoginProvider
		if e := json.Unmarshal(r.Config, &p); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, err
}
func (s *Store) SaveProvider(ctx context.Context, p LoginProvider) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&providerRecord{ID: p.ID, Config: data}).Error
}
func (s *Store) DeleteProvider(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where(&providerRecord{ID: id}).Delete(&providerRecord{}).Error
}
