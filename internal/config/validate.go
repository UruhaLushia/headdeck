package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (c *Config) Validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	if unit := os.Getenv("HEADDECK_SERVICE_UNIT"); unit != "" && c.Features.ServiceUnit != unit {
		add("service_unit must match the root-owned systemd unit (%s)", unit)
	}

	if c.IPC.Listen == "" {
		add("ipc.listen must not be empty")
	} else if !filepath.IsAbs(c.IPC.Listen) {
		add("ipc.listen %q must be an absolute path", c.IPC.Listen)
	} else if strings.HasPrefix(filepath.Clean(c.IPC.Listen), "/tmp/") {
		add("ipc.listen %q must not live under /tmp: the path is world writable", c.IPC.Listen)
	}
	if _, err := c.SocketFileMode(); err != nil {
		add("%v", err)
	}
	u, err := url.Parse(c.Headscale.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		add("headscale.base_url must be an HTTP(S) URL without credentials")
	}

	if strings.ContainsAny(strings.TrimSpace(c.Headscale.APIKey), "\r\n") {
		add("headscale.api_key must be a single line")
	}
	if c.Headscale.Timeout < 0 {
		add("headscale.timeout must not be negative")
	}
	if c.Headscale.MaxRetries < 0 || c.Headscale.MaxRetries > 10 {
		add("headscale.max_retries must be between 0 and 10")
	}
	switch c.Features.ServiceControlDriver {
	case "", "auto", "systemd", "none":
	default:
		add("features.service_control_driver %q is not one of auto, systemd, none",
			c.Features.ServiceControlDriver)
	}
	if c.Features.HeadscaleServiceControl && c.Features.ServiceUnit == "" {
		add("features.service_unit must be set when headscale_service_control is enabled")
	}
	if c.Features.ServiceUnit != "" && !ValidServiceUnit(c.Features.ServiceUnit) {
		add("features.service_unit must be a valid .service unit name")
	}
	if err := c.validateWeb(); err != nil {
		add("%v", err)
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
}

func ValidServiceUnit(unit string) bool {
	if len(unit) > 255 || len(unit) <= len(".service") || !strings.HasSuffix(unit, ".service") {
		return false
	}
	for _, c := range unit {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("@_.:-", c) {
			continue
		}
		return false
	}
	return true
}

func (c *Config) validateWeb() error {
	web := c.Server.Web
	if web.DefaultRole != "pending" && web.DefaultRole != "member" {
		return fmt.Errorf("server.web.default_role must be pending or member")
	}
	if err := c.validateGitHub(); err != nil {
		return err
	}
	if err := c.validateOIDC(); err != nil {
		return err
	}
	if web.PublicURL != "" {
		u, err := url.Parse(web.PublicURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !validWebPath(u) {
			return fmt.Errorf("server.web.public_url must be an HTTP(S) URL with a simple deployment path")
		}
	}
	if !web.Enabled {
		return c.validateOIDCServer()
	}
	if web.Listen == "" {
		return fmt.Errorf("server.web.listen must be set when the web listener is enabled")
	}
	if _, _, err := net.SplitHostPort(web.Listen); err != nil {
		return fmt.Errorf("server.web.listen %q must be host:port", web.Listen)
	}
	if web.SessionTTL <= 0 {
		return fmt.Errorf("server.web.session_ttl must be positive")
	}
	if (web.TLSCert == "") != (web.TLSKey == "") {
		return fmt.Errorf("server.web.tls_cert and server.web.tls_key must be set together")
	}
	if web.TLSCert == "" && !isLoopbackListen(web.Listen) {
		return fmt.Errorf("server.web.listen %q is not loopback: enable TLS or terminate it in a trusted proxy", web.Listen)
	}
	proxyTLS := strings.HasPrefix(web.PublicURL, "https://") && len(web.TrustedProxies) > 0 && isLoopbackListen(web.Listen)
	if web.TLSCert == "" && web.CookieSecure && !proxyTLS {
		return fmt.Errorf("server.web.cookie_secure requires TLS")
	}
	if !isLoopbackListen(web.Listen) && !web.CookieSecure {
		return fmt.Errorf("server.web.cookie_secure must be true for a non-loopback listener")
	}
	for _, cidr := range web.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("server.web.trusted_proxies entry %q is not a CIDR", cidr)
		}
	}
	if web.HeadscaleProxy {
		u, err := url.Parse(web.PublicURL)
		if err != nil || u.Host == "" || u.Path != "" && u.Path != "/" {
			return fmt.Errorf("headscale proxy requires server.web.public_url at the domain root")
		}
	}
	return c.validateOIDCServer()
}

func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}
	return ip.IsLoopback()
}
