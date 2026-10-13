package config

import "fmt"

func SecretSetting(key string) bool {
	return key == "headscale.api_key" || key == "server.web.oidc.client_secret" || key == "server.web.github.client_secret" || key == "server.web.oidc_server.client_secret"
}

func (c WebConfig) TokenLoginEnabled() bool {
	return !c.OIDC.DisableTokenLogin && !c.GitHub.DisableTokenLogin
}

func (c *Config) validateGitHub() error {
	g := c.Server.Web.GitHub
	if !g.Enabled {
		return nil
	}
	if c.Server.Web.PublicURL == "" || g.ClientID == "" || g.ClientSecret == "" {
		return fmt.Errorf("GitHub login requires a public URL, client ID and client secret")
	}
	if len(g.ClientID) > 2048 || len(g.ClientSecret) > 8192 {
		return fmt.Errorf("GitHub login settings exceed size limits")
	}
	return nil
}
