package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func settingLabel(key string) string {
	if strings.HasPrefix(key, "hs.") {
		return managedLabel(key)
	}
	if strings.HasPrefix(key, "dns.") {
		return dnsLabel(key)
	}
	labels := map[string]string{
		"features.headscale_service_control":    "Service control",
		"features.service_control_driver":       "Service driver",
		"features.service_unit":                 "Service unit",
		"server.diagnostics":                    "Debug",
		"server.log_format":                     "Logging · Format",
		"server.log_level":                      "Logging · Level",
		"server.web.enabled":                    "Web · Enabled",
		"server.web.password_login":             "Web · Password login",
		"server.web.default_role":               "Web · Default registration role",
		"server.web.public_url":                 "Web · Public URL",
		"server.web.listen":                     "Web · Listen address",
		"server.web.cookie_secure":              "Web · Secure cookies",
		"server.web.session_ttl":                "Web · Session duration",
		"server.web.session_key_file":           "Web · Session key file",
		"server.web.token_file":                 "Web · Token file",
		"server.web.tls_cert":                   "Web · TLS certificate",
		"server.web.tls_key":                    "Web · TLS key",
		"server.web.trusted_proxies":            "Web · Trusted proxies",
		"server.web.oidc.enabled":               "Login · Single sign-on",
		"server.web.github.enabled":             "GitHub · Enabled",
		"server.web.github.client_id":           "GitHub · Client ID",
		"server.web.github.client_secret":       "GitHub · Client secret",
		"server.web.github.auto_register":       "GitHub · Auto-register",
		"server.web.github.disable_token_login": "GitHub · Disable token login",
		"server.web.oidc.name":                  "Login · Provider name",
		"server.web.oidc.issuer":                "Login · Provider URL",
		"server.web.oidc.client_id":             "Login · Client ID",
		"server.web.oidc.client_secret":         "Login · Client secret",
		"server.web.oidc.auto_register":         "Login · Auto-register",
		"server.web.oidc.disable_token_login":   "Login · Disable token login",
	}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

func settingHelp(key string) string {
	if strings.HasPrefix(key, "dns.") {
		return dnsHelp(key)
	}
	if len(settingChoices(key)) != 0 {
		return "Press Enter or click to choose an option."
	}
	switch key {
	case "server.web.session_ttl":
		return "Use a duration, for example 8s, 30m or 12h."
	case "server.web.trusted_proxies":
		return "Only accept client IPs forwarded by these proxies. Example: 127.0.0.1/32."
	}
	return "Press Enter to edit this setting."
}

func inputLine(text string, position, width int, secret bool) string {
	value := []rune(text)
	if secret {
		value = []rune(strings.Repeat("•", len(value)))
	}
	cursor := max(0, min(position, len(value)))
	before := string(value[:cursor])
	if lipgloss.Width(before) >= width {
		before = ansi.TruncateLeft(before, lipgloss.Width(before)-width+1, "")
	}
	current, after := " ", ""
	if cursor < len(value) {
		current, after = string(value[cursor]), string(value[cursor+1:])
	}
	caret := lipgloss.NewStyle().Foreground(lipgloss.Color("#0F172A")).Background(accent).Render(current)
	return titleStyle.Render(before) + caret + titleStyle.Render(ansi.Truncate(after, max(0, width-lipgloss.Width(before)-1), ""))
}
