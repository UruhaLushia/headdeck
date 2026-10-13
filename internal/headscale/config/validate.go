package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func domain(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			valid := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
			if !valid {
				return false
			}
		}
	}
	return true
}

func resolver(value string) bool {
	if _, err := netip.ParseAddr(value); err == nil {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}

func Validate(c protocol.DNSConfig, serverURL string) error {
	if c.MagicDNS && !domain(c.BaseDomain) {
		return fmt.Errorf("MagicDNS requires a valid base domain")
	}
	if c.BaseDomain != "" && !domain(c.BaseDomain) {
		return fmt.Errorf("invalid base domain")
	}
	u, err := url.Parse(serverURL)
	if err == nil && strings.EqualFold(c.BaseDomain, u.Hostname()) {
		return fmt.Errorf("base domain must differ from the Headscale server domain")
	}
	if len(c.Nameservers.Global)+len(c.Nameservers.Split)+len(c.SearchDomains)+len(c.ExtraRecords) > 1000 {
		return fmt.Errorf("too many DNS entries")
	}
	if c.OverrideLocalDNS && len(c.Nameservers.Global) == 0 {
		return fmt.Errorf("DNS override requires a global nameserver")
	}
	for _, ns := range c.Nameservers.Global {
		if !resolver(ns) {
			return fmt.Errorf("invalid nameserver %q", ns)
		}
	}
	for name, servers := range c.Nameservers.Split {
		if !domain(name) || len(servers) == 0 || len(servers) > 100 {
			return fmt.Errorf("invalid split DNS domain or empty nameservers")
		}
		for _, ns := range servers {
			if !resolver(ns) {
				return fmt.Errorf("invalid nameserver %q", ns)
			}
		}
	}
	for _, name := range c.SearchDomains {
		if !domain(name) {
			return fmt.Errorf("invalid search domain %q", name)
		}
	}
	seen := map[string]bool{}
	for _, record := range c.ExtraRecords {
		ip, err := netip.ParseAddr(record.Value)
		if !domain(strings.TrimSuffix(record.Name, ".")) || err != nil || record.Type != "A" && record.Type != "AAAA" || record.Type == "A" && !ip.Is4() || record.Type == "AAAA" && !ip.Is6() {
			return fmt.Errorf("invalid DNS record; use a domain and an A/AAAA address")
		}
		key := strings.ToLower(record.Name) + "/" + record.Type + "/" + record.Value
		if seen[key] {
			return fmt.Errorf("duplicate DNS record")
		}
		seen[key] = true
	}
	return nil
}
