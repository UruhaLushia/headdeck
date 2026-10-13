package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeCannotChangeInstallationSettings(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, key := range []string{"headscale.api_key", "headscale.base_url", "headscale.timeout", "headscale.max_retries", "headscale.insecure_skip_verify", "features.service_unit", "server.web.session_key_file", "server.web.token_file", "server.web.tls_cert", "server.web.tls_key", "server.web.oidc_server.signing_key_file"} {
		if _, err := s.UpdateValidated(ctx, map[string]string{key: "/etc/security-target"}, func(*Config) error {
			t.Fatal("protected setting reached validation side effects")
			return nil
		}); err == nil {
			t.Fatal("runtime changed protected setting", key)
		}
	}
	c := Default()
	c.Server.Web.SessionKeyFile = filepath.Join(dir, "local-key")
	if err := Save(filepath.Join(dir, "settings.db"), c); err != nil {
		t.Fatal("local installation configuration rejected", err)
	}
	for _, item := range Settings(Default()) {
		if ProtectedSetting(item.Key) {
			t.Fatal("installation-only setting exposed as editable", item.Key)
		}
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(dir, "unsafe.db")); err == nil {
		t.Fatal("database created in an attacker-writable directory")
	}
}

func TestSettingsChangesIncludeSecrets(t *testing.T) {
	current := Default()
	for _, key := range []string{"headscale.api_key", "server.web.oidc.client_secret", "server.web.github.client_secret", "server.web.oidc_server.client_secret"} {
		next := Default()
		if err := set(next, key, "new-secret"); err != nil {
			t.Fatal(err)
		}
		if !SettingsChanged(next, current) {
			t.Fatal("secret-only update was ignored", key)
		}
	}
	if SettingsChanged(Default(), current) {
		t.Fatal("unchanged settings require a reload")
	}
}

func TestValidateRejectsUnsafePaths(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"relative socket", func(c *Config) { c.IPC.Listen = "ipc.sock" }, "absolute"},
		{"socket under tmp", func(c *Config) { c.IPC.Listen = "/tmp/headdeck.sock" }, "/tmp"},
		{"web on a public interface without TLS", func(c *Config) {
			c.Server.Web.Enabled = true
			c.Server.Web.Listen = "0.0.0.0:8088"
		}, "not loopback"},
		{"web on a public interface without a secure cookie", func(c *Config) {
			c.Server.Web.Enabled = true
			c.Server.Web.Listen = "0.0.0.0:8443"
			c.Server.Web.TLSCert = "/etc/headdeck/tls.crt"
			c.Server.Web.TLSKey = "/etc/headdeck/tls.key"
		}, "cookie_secure"},
		{"web with half a TLS pair", func(c *Config) {
			c.Server.Web.Enabled = true
			c.Server.Web.TLSCert = "/etc/headdeck/tls.crt"
		}, "must be set together"},
		{"web with a bad proxy cidr", func(c *Config) {
			c.Server.Web.Enabled = true
			c.Server.Web.TrustedProxies = []string{"not-a-cidr"}
		}, "CIDR"},
		{"bad socket mode", func(c *Config) { c.IPC.SocketMode = "99" }, "octal"},
		{"negative retries", func(c *Config) { c.Headscale.MaxRetries = -1 }, "max_retries"},
		{"unknown driver", func(c *Config) { c.Features.ServiceControlDriver = "docker" }, "service_control_driver"},
		{"unsafe registration role", func(c *Config) { c.Server.Web.DefaultRole = "owner" }, "default_role"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			tc.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("configuration was accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestWebListenerValidation(t *testing.T) {
	// Disabled by default, and the defaults must pass validation.
	if Default().Server.Web.Enabled || Default().Server.Web.PasswordLogin {
		t.Fatal("Web and password login must be off by default")
	}

	ok := Default()
	ok.Server.Web.Enabled = true
	if err := ok.Validate(); err == nil {
		t.Fatal("Web enabled without Headscale login takeover")
	}
	configureBroker(ok)
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}

	tls := Default()
	tls.Server.Web.Enabled = true
	tls.Server.Web.Listen = "0.0.0.0:8443"
	tls.Server.Web.TLSCert = "/etc/headdeck/tls.crt"
	tls.Server.Web.TLSKey = "/etc/headdeck/tls.key"
	tls.Server.Web.CookieSecure = true
	configureBroker(tls)
	if err := tls.Validate(); err != nil {
		t.Fatalf("a TLS web listener must be valid: %v", err)
	}
}

func TestGitHubLoginValidation(t *testing.T) {
	cfg := Default()
	cfg.Server.Web.Enabled = true
	cfg.Server.Web.PublicURL = "https://headdeck.example/headdeck"
	cfg.Server.Web.TrustedProxies = []string{"127.0.0.1/32"}
	cfg.Server.Web.CookieSecure = true
	cfg.Server.Web.GitHub = GitHubConfig{Enabled: true, ClientID: "client", ClientSecret: "secret", AutoRegister: true}
	configureBroker(cfg)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Server.Web.CookieSecure = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("insecure login accepted")
	}
	cfg.Server.Web.CookieSecure = true
	cfg.Server.Web.GitHub.ClientSecret = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("missing GitHub secret accepted")
	}
}

func TestServiceUnitPinnedBySystemd(t *testing.T) {
	t.Setenv("HEADDECK_SERVICE_UNIT", "headscale.service")
	cfg := Default()
	cfg.Features.ServiceUnit = "other.service"
	if err := cfg.Validate(); err == nil {
		t.Fatal("settings widened the installed service scope")
	}
}

func TestOIDCServerPrerequisites(t *testing.T) {
	c := Default()
	if c.Server.Web.OIDCServer.Enabled {
		t.Fatal("broker must stay disabled while Web is disabled")
	}
	w := &c.Server.Web
	w.Enabled = true
	w.PublicURL = "https://headdeck.example"
	w.CookieSecure = true
	w.TrustedProxies = []string{"127.0.0.1/32"}
	w.GitHub = GitHubConfig{Enabled: true, ClientID: "github", ClientSecret: "private"}
	w.OIDCServer.Enabled = true
	w.OIDCServer.ClientSecret = strings.Repeat("s", 32)
	w.OIDCServer.RedirectURI = "https://headdeck.example/oidc/callback"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Server.Web.CookieSecure = false },
		func(c *Config) { c.Server.Web.OIDCServer.ClientSecret = "short" },
		func(c *Config) { c.Server.Web.OIDCServer.RedirectURI = "http://headscale.example/oidc/callback" },
		func(c *Config) { c.Server.Web.OIDCServer.TokenTTL = 25 * time.Hour },
		func(c *Config) { c.Server.Web.OIDCServer.SigningKeyFile = "/tmp/key.pem" },
	} {
		next := *c
		mutate(&next)
		if err := next.Validate(); err == nil {
			t.Fatal("insecure OIDC settings accepted")
		}
	}
	c.Server.Web.GitHub.Enabled = false
	if err := c.Validate(); err != nil {
		t.Fatal("broker must accept other Headdeck login methods", err)
	}
	c.Server.Web.Enabled = false
	if err := c.Validate(); err != nil {
		t.Fatal("broker settings must be preparable before enabling Web", err)
	}
	for _, item := range Settings(c) {
		if item.Key == "server.web.oidc_server.client_secret" && (!item.Secret || item.Value != "" || !item.Configured) {
			t.Fatal("OIDC client secret exposed")
		}
	}
}

func TestManagedOIDCSettings(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "settings.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secret := initial.Server.Web.OIDCServer.ClientSecret
	if len(secret) < 32 {
		t.Fatal("OIDC secret was not generated")
	}
	for _, key := range []string{"enabled", "client_id", "client_secret", "redirect_uri", "token_ttl"} {
		if _, err := s.Update(ctx, map[string]string{"server.web.oidc_server." + key: "manual"}); err == nil {
			t.Fatal("managed OIDC setting was editable", key)
		}
	}
	if _, err := s.Update(ctx, map[string]string{"server.web.headscale_proxy": "false"}); err == nil {
		t.Fatal("mandatory Headscale proxy was editable")
	}
	configured, err := s.Update(ctx, map[string]string{
		"server.web.enabled": "true", "server.web.public_url": "https://control.example",
		"server.web.cookie_secure": "true", "server.web.trusted_proxies": `["127.0.0.1/32"]`,
	})
	if err != nil {
		t.Fatal("Web required manual broker configuration", err)
	}
	o := configured.Server.Web.OIDCServer
	if !o.Enabled || o.ClientID != "headscale" || o.RedirectURI != "https://control.example/oidc/callback" || o.TokenTTL != time.Hour || o.ClientSecret != secret {
		t.Fatal("incorrect managed OIDC configuration")
	}
	for _, item := range Settings(configured) {
		if strings.HasPrefix(item.Key, "server.web.oidc_server.") || item.Key == "server.web.headscale_proxy" || strings.HasPrefix(item.Key, "headscale.") {
			t.Fatal("managed OIDC configuration exposed", item.Key)
		}
	}
	if !configured.Server.Web.HeadscaleProxy {
		t.Fatal("Headscale proxy was not enabled")
	}
	if _, err := s.Update(ctx, map[string]string{"headscale.base_url": "https://control.example"}); err == nil {
		t.Fatal("managed upstream could be overridden")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	reloaded, err := s.Load(ctx)
	if err != nil || reloaded.Server.Web.OIDCServer != o {
		t.Fatal("managed OIDC configuration changed after restart", err)
	}
}

func configureBroker(c *Config) {
	w := &c.Server.Web
	w.PublicURL = "https://headdeck.example"
	w.CookieSecure = true
	w.TrustedProxies = []string{"127.0.0.1/32"}
	w.OIDCServer.Enabled = true
	w.OIDCServer.ClientSecret = strings.Repeat("s", 32)
	w.OIDCServer.RedirectURI = "https://headdeck.example/oidc/callback"
}
