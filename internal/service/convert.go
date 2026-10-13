package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/headscale"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func toNode(n headscale.Node) protocol.Node {
	out := protocol.Node{
		ID:              n.ID,
		Name:            n.Name,
		GivenName:       n.GivenName,
		IPAddresses:     n.IPAddresses,
		Online:          n.Online,
		LastSeen:        formatTimePtr(n.LastSeen),
		Expiry:          formatTimePtr(n.Expiry),
		RegisterMethod:  normalizeRegisterMethod(n.RegisterMethod),
		AvailableRoutes: n.AvailableRoutes,
		ApprovedRoutes:  n.ApprovedRoutes,
		EnabledRoutes:   n.SubnetRoutes,
		CreatedAt:       formatTime(n.CreatedAt),
		ForcedTags:      n.ForcedTags,
		ValidTags:       n.ValidTags,
		InvalidTags:     n.InvalidTags,
	}
	if n.GivenName == "" {
		out.GivenName = n.Name
	}
	if n.Tags != nil {
		out.ForcedTags = n.Tags
		out.ValidTags = n.Tags
	}
	if n.User != nil {
		out.User = protocol.Ref{ID: n.User.ID, Name: n.User.Name}
	}
	if n.Expiry != nil && !n.Expiry.IsZero() {
		out.Expired = n.Expiry.Before(time.Now())
	}
	if n.PreAuthKey != nil {
		out.Ephemeral = n.PreAuthKey.Ephemeral
	}
	return out
}

func normalizeRegisterMethod(raw string) string {
	raw = strings.TrimPrefix(raw, "REGISTER_METHOD_")
	if raw == "UNSPECIFIED" {
		return ""
	}
	return strings.ToLower(raw)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func Describe(n protocol.Node) string {
	if n.GivenName != "" {
		return fmt.Sprintf("node %s (%s)", n.ID, n.GivenName)
	}
	return "node " + n.ID
}
