package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OIDCServerConfig struct {
	Enabled        bool          `json:"enabled"`
	ClientID       string        `json:"client_id"`
	ClientSecret   string        `json:"client_secret"`
	RedirectURI    string        `json:"redirect_uri"`
	SigningKeyFile string        `json:"signing_key_file"`
	TokenTTL       time.Duration `json:"token_ttl"`
}

func (w WebConfig) OIDCIssuer() string { return strings.TrimRight(w.PublicURL, "/") + "/oidc" }

func (c *Config) manageOIDC() {
	c.Headscale.Timeout = 8 * time.Second
	c.Headscale.MaxRetries = 2
	c.Headscale.InsecureSkipVerify = false
	w := &c.Server.Web
	w.HeadscaleProxy = true
	w.OIDCServer.Enabled = w.Enabled
	w.OIDCServer.ClientID = "headscale"
	w.OIDCServer.RedirectURI = strings.TrimRight(w.PublicURL, "/") + "/oidc/callback"
	w.OIDCServer.TokenTTL = time.Hour
}

type settingRecord struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

func (settingRecord) TableName() string { return "settings" }

func (s *Store) ensureOIDCSecret() error {
	db, err := database.Open(s.db)
	if err != nil {
		return err
	}
	const key = "server.web.oidc_server.client_secret"
	var stored settingRecord
	err = db.Where(&settingRecord{Key: key}).Take(&stored).Error
	if err == nil && stored.Value != "" {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	secret := rand.Text() + rand.Text()
	if err == nil {
		return db.Model(&settingRecord{}).Where(&settingRecord{Key: key}).Where("value = ?", "").Update("value", secret).Error
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&settingRecord{Key: key, Value: secret}).Error
}

func (c *Config) validateOIDCServer() error {
	w := c.Server.Web
	o := w.OIDCServer
	if !o.Enabled {
		if w.Enabled {
			return fmt.Errorf("web requires Headdeck to manage Headscale login; configure it through the CLI or TUI first")
		}
		return nil
	}
	if !w.CookieSecure || !strings.HasPrefix(w.PublicURL, "https://") {
		return fmt.Errorf("headscale login requires HTTPS and secure cookies")
	}
	if o.ClientID == "" || len(o.ClientID) > 256 || len(o.ClientSecret) < 32 || len(o.ClientSecret) > 8192 {
		return fmt.Errorf("headscale login requires a client ID and a client secret of at least 32 characters")
	}
	u, err := url.Parse(o.RedirectURI)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path == "" {
		return fmt.Errorf("headscale login requires an exact HTTPS callback URL")
	}
	if w.HeadscaleProxy && o.RedirectURI != strings.TrimRight(w.PublicURL, "/")+"/oidc/callback" {
		return fmt.Errorf("headscale proxy requires the OIDC callback on the public domain root")
	}
	if !filepath.IsAbs(o.SigningKeyFile) || filepath.Clean(o.SigningKeyFile) != o.SigningKeyFile || strings.HasPrefix(o.SigningKeyFile, "/tmp/") {
		return fmt.Errorf("OIDC signing key must use an absolute protected path")
	}
	if o.TokenTTL < 5*time.Minute || o.TokenTTL > 24*time.Hour {
		return fmt.Errorf("OIDC token duration must be between 5m and 24h")
	}
	return nil
}
