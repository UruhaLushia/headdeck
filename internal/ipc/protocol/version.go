// Protocol must remain independent of Headscale and service control.
package protocol

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const ProtocolVersion = 1

var SupportedVersions = VersionRange{Min: ProtocolVersion, Max: ProtocolVersion}

type VersionRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

func (r VersionRange) Contains(v int) bool { return v >= r.Min && v <= r.Max }

func (r VersionRange) String() string {
	if r.Min == r.Max {
		return strconv.Itoa(r.Min)
	}
	return fmt.Sprintf("%d-%d", r.Min, r.Max)
}

func Negotiate(client, server VersionRange) (version int, ok bool) {
	if client.Min <= 0 || client.Max < client.Min || server.Min <= 0 || server.Max < server.Min {
		return 0, false
	}
	low, high := client.Min, client.Max
	if server.Min > low {
		low = server.Min
	}
	if server.Max < high {
		high = server.Max
	}
	if low > high {
		return 0, false
	}
	return high, true
}

const (
	FeatureNodesRead      = "nodes.read"
	FeatureNodesWrite     = "nodes.write"
	FeatureNodesDelete    = "nodes.delete"
	FeatureUsersRead      = "users.read"
	FeatureSystemRead     = "system.read"
	FeatureSystemOps      = "system.operations"
	FeatureServiceRestart = "headscale.restart"
	FeatureEventsSSE      = "events.sse"
	FeatureKeysRead       = "keys.read"
	FeatureKeysWrite      = "keys.write"
	FeatureRoutesRead     = "routes.read"
	FeatureRoutesWrite    = "routes.write"
)

var AllFeatures = []string{
	"headscale.config.read", "headscale.config.write", "policy.read", "policy.write",
	"dns.read", "dns.write",
	"users.write", "accounts.read", "accounts.write",
	"settings.read", "settings.write",
	FeatureSystemRead,
	FeatureNodesRead,
	FeatureNodesWrite,
	FeatureNodesDelete,
	FeatureUsersRead,
	FeatureKeysRead,
	FeatureKeysWrite,
	FeatureRoutesRead,
	FeatureRoutesWrite,
	FeatureServiceRestart,
	"headscale.start", "headscale.stop", "headscale.reload",
	FeatureEventsSSE,
	FeatureSystemOps,
}

func SortedFeatures(features []string) string {
	out := append([]string(nil), features...)
	sort.Strings(out)
	return strings.Join(out, ",")
}
