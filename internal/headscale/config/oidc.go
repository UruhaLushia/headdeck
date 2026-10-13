package config

import (
	"crypto/subtle"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/config"
	"go.yaml.in/yaml/v3"
)

type oidcConfig struct {
	Issuer           string            `yaml:"issuer"`
	ClientID         string            `yaml:"client_id"`
	ClientSecret     string            `yaml:"client_secret"`
	ClientSecretPath string            `yaml:"client_secret_path,omitempty"`
	Scope            []string          `yaml:"scope"`
	AllowedUsers     []string          `yaml:"allowed_users"`
	AllowedDomains   []string          `yaml:"allowed_domains"`
	AllowedGroups    []string          `yaml:"allowed_groups"`
	ExtraParams      map[string]string `yaml:"extra_params"`
	PKCE             struct {
		Enabled bool   `yaml:"enabled"`
		Method  string `yaml:"method"`
	} `yaml:"pkce"`
}

func (s *Store) CheckOIDC(w config.WebConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !writable(s.path, s.managedDir) {
		return fmt.Errorf("headscale configuration management must be authorized first")
	}
	data, err := readFile(s.path)
	if err != nil {
		return err
	}
	var document struct {
		listenerConfig `yaml:",inline"`
		ServerURL      string      `yaml:"server_url"`
		OIDC           *oidcConfig `yaml:"oidc"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	if document.OIDC == nil {
		return fmt.Errorf("headscale OIDC has not been taken over")
	}
	address, err := document.localAddress()
	if err != nil {
		return err
	}
	if document.ListenAddr != address {
		return fmt.Errorf("headscale listen_addr must use its managed loopback address")
	}
	if w.HeadscaleProxy && strings.TrimRight(document.ServerURL, "/") != strings.TrimRight(w.PublicURL, "/") {
		return fmt.Errorf("headscale server_url must match the Headdeck public URL when proxying")
	}
	current := document.OIDC
	wanted := w.OIDCServer
	if current.Issuer != w.OIDCIssuer() || current.ClientID != wanted.ClientID || current.ClientSecretPath != "" ||
		subtle.ConstantTimeCompare([]byte(current.ClientSecret), []byte(wanted.ClientSecret)) != 1 ||
		!current.PKCE.Enabled || current.PKCE.Method != "S256" {
		return fmt.Errorf("headscale OIDC does not match Headdeck settings")
	}
	if len(current.AllowedUsers)+len(current.AllowedDomains)+len(current.AllowedGroups)+len(current.ExtraParams) != 0 {
		return fmt.Errorf("headscale login authorization must be managed by Headdeck")
	}
	if len(current.Scope) != 0 && (!slices.Contains(current.Scope, "openid") || !slices.Contains(current.Scope, "profile")) {
		return fmt.Errorf("headscale OIDC requires openid and profile scopes")
	}
	callback := strings.TrimRight(document.ServerURL, "/") + "/oidc/callback"
	u, err := url.Parse(callback)
	if err != nil || u.Scheme != "https" || u.Host == "" || callback != wanted.RedirectURI {
		return fmt.Errorf("headscale callback does not match Headdeck settings")
	}
	return nil
}
