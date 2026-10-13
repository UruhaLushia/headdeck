package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func (c *Config) validateOIDC() error {
	web := c.Server.Web
	o := web.OIDC
	if !o.Enabled && !web.GitHub.Enabled {
		return nil
	}
	if o.Enabled && (web.PublicURL == "" || o.ClientID == "") {
		return fmt.Errorf("OIDC requires server.web.public_url and a client ID")
	}
	urls := []string{web.PublicURL}
	if o.Enabled {
		urls = append(urls, o.Issuer)
	}
	for _, value := range urls {
		u, err := url.Parse(value)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("login requires valid public and issuer URLs without credentials")
		}
		ip := net.ParseIP(u.Hostname())
		loopback := u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
		if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
			return fmt.Errorf("login requires HTTPS except on loopback")
		}
	}
	if strings.HasPrefix(web.PublicURL, "https://") && !web.CookieSecure {
		return fmt.Errorf("login over HTTPS requires secure cookies")
	}
	if len(o.Name) > 80 || len(o.ClientID) > 2048 || len(o.ClientSecret) > 8192 {
		return fmt.Errorf("OIDC settings exceed size limits")
	}
	return nil
}
