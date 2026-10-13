package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

var managedFields = map[string]string{
	"metrics_listen_addr": "address",
	"prefixes.v4":         "prefix4", "prefixes.v6": "prefix6",
	"prefixes.allocation": "allocation", "disable_check_updates": "bool",
	"log.level": "level", "log.format": "format", "logtail.enabled": "bool",
	"taildrop.enabled": "bool", "auto_update.enabled": "bool",
	"node.expiry": "expiry", "node.ephemeral.inactivity_timeout": "duration",
	"ephemeral_node_inactivity_timeout": "duration",
	"derp.urls":                         "urls", "derp.auto_update_enabled": "bool", "derp.update_frequency": "duration",
	"derp.server.enabled": "bool", "derp.server.region_id": "region",
	"derp.server.region_code": "text", "derp.server.region_name": "text",
	"derp.server.verify_clients": "bool", "derp.server.stun_listen_addr": "address",
	"derp.server.automatically_add_embedded_derp_region": "bool",
	"derp.server.ipv4": "ipv4", "derp.server.ipv6": "ipv6",
}

func fieldNode(root *yaml.Node, key string) *yaml.Node {
	for _, part := range strings.Split(key, ".") {
		if root == nil || root.Kind != yaml.MappingNode {
			return nil
		}
		root = child(root, part)
	}
	return root
}

func prefixParent(root *yaml.Node, key string) *yaml.Node {
	if key == "prefixes.v4" || key == "prefixes.v6" {
		if parent := fieldNode(root, "prefixes"); parent != nil && parent.Kind == yaml.MappingNode {
			return parent
		}
	}
	return nil
}

func fieldValue(n *yaml.Node) (string, error) {
	if n.Kind == yaml.ScalarNode {
		return n.Value, nil
	}
	var values []string
	if err := n.Decode(&values); err != nil {
		return "", err
	}
	data, err := json.Marshal(values)
	return string(data), err
}

func parseField(key, value string) (*yaml.Node, error) {
	tag := "!!str"
	switch managedFields[key] {
	case "bool":
		if value != "true" && value != "false" {
			return nil, fmt.Errorf("%s requires true or false", key)
		}
		tag = "!!bool"
	case "allocation", "level", "format":
		choices := map[string][]string{"allocation": {"sequential", "random"}, "level": {"panic", "fatal", "error", "warn", "info", "debug", "trace"}, "format": {"text", "json"}}
		valid := false
		for _, option := range choices[managedFields[key]] {
			valid = valid || option == value
		}
		if !valid {
			return nil, fmt.Errorf("invalid value for %s", key)
		}
	case "region":
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 || id > 65535 {
			return nil, fmt.Errorf("DERP region ID must be between 1 and 65535")
		}
		tag = "!!int"
	case "duration", "expiry":
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 0 || duration == 0 && managedFields[key] != "expiry" {
			return nil, fmt.Errorf("invalid duration for %s", key)
		}
	case "urls":
		var urls []string
		if err := json.Unmarshal([]byte(value), &urls); err != nil || urls == nil {
			return nil, fmt.Errorf("DERP URLs require a JSON array")
		}
		for _, address := range urls {
			if !validHTTPURL(address) {
				return nil, fmt.Errorf("invalid DERP URL")
			}
		}
		var n yaml.Node
		if err := n.Encode(urls); err != nil {
			return nil, err
		}
		return &n, nil
	case "address":
		_, port, err := net.SplitHostPort(value)
		n, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("%s requires host:port", key)
		}
	case "prefix4", "prefix6":
		if value != "" {
			prefix, err := netip.ParsePrefix(value)
			if err != nil || prefix.Addr().Is4() != (managedFields[key] == "prefix4") || prefix.Addr().Is4In6() {
				return nil, fmt.Errorf("%s requires a valid IPv%s CIDR", key, strings.TrimPrefix(managedFields[key], "prefix"))
			}
		}
	case "ipv4", "ipv6":
		if value != "" {
			address, err := netip.ParseAddr(value)
			if err != nil || address.Is4() != (managedFields[key] == "ipv4") {
				return nil, fmt.Errorf("invalid address for %s", key)
			}
		}
	case "text":
		if strings.TrimSpace(value) == "" || len(value) > 128 {
			return nil, fmt.Errorf("invalid text for %s", key)
		}
	default:
		return nil, fmt.Errorf("unknown or protected Headscale field %s", key)
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}, nil
}

func validHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}
