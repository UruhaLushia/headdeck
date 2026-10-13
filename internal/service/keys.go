package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/headscale"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

const maxPreAuthKeyLifetime = 10 * 365 * 24 * time.Hour

func (s *Service) PreAuthKeys(ctx context.Context, user string) ([]protocol.PreAuthKey, error) {
	keys, err := s.hs.ListPreAuthKeys(ctx)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := make([]protocol.PreAuthKey, 0, len(keys))
	for _, k := range keys {
		if user != "" && (k.User == nil || k.User.Name != user) {
			continue
		}
		out = append(out, toPreAuthKey(k, false))
	}
	return out, nil
}

func (s *Service) CreatePreAuthKey(ctx context.Context, req protocol.CreatePreAuthKeyRequest) (*protocol.PreAuthKey, error) {
	user := strings.TrimSpace(req.User)
	if user == "" {
		return nil, protocol.NewError(protocol.CodeBadRequest, "user is required")
	}
	userID, err := s.resolveUserID(ctx, user)
	if err != nil {
		return nil, err
	}
	if err := requireOwner(ctx, userID); err != nil {
		return nil, err
	}
	if p, ok := principal.FromContext(ctx); ok && !p.IsAdmin() && len(req.ACLTags) != 0 {
		return nil, protocol.NewError(protocol.CodePermissionDenied, "only administrators may create tagged keys")
	}
	expiration, err := parseExpiration(req.Expiration)
	if err != nil {
		return nil, err
	}
	key, err := s.hs.CreatePreAuthKey(ctx, headscale.CreatePreAuthKeyRequest{
		User:       userID,
		Reusable:   req.Reusable,
		Ephemeral:  req.Ephemeral,
		Expiration: expiration,
		ACLTags:    req.ACLTags,
	})
	if err != nil {
		return nil, headscaleError(err)
	}
	s.publish(protocol.EventKeysChanged, key.ID)
	// The secret is returned once, exactly as Headscale does.
	out := toPreAuthKey(*key, true)
	return &out, nil
}

func (s *Service) DeletePreAuthKey(ctx context.Context, id string) error {
	if err := s.requireKeyOwner(ctx, id); err != nil {
		return err
	}
	if err := s.hs.DeletePreAuthKey(ctx, id); err != nil {
		return headscaleError(err)
	}
	s.publish(protocol.EventKeysChanged, id)
	return nil
}

func (s *Service) ExpirePreAuthKey(ctx context.Context, id string) error {
	if err := s.requireKeyOwner(ctx, id); err != nil {
		return err
	}
	if err := s.hs.ExpirePreAuthKey(ctx, id); err != nil {
		return headscaleError(err)
	}
	s.publish(protocol.EventKeysChanged, id)
	return nil
}

// parseExpiration accepts either an RFC3339 timestamp or a duration such as
// "24h" or "90d", and refuses anything beyond ten years.
func parseExpiration(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if d, err := parseLifetime(raw); err == nil {
		if d <= 0 {
			return "", protocol.NewError(protocol.CodeBadRequest, "expiration must be in the future")
		}
		if d > maxPreAuthKeyLifetime {
			return "", protocol.NewError(protocol.CodeBadRequest,
				"expiration must be at most %s", maxPreAuthKeyLifetime)
		}
		return time.Now().Add(d).UTC().Format(time.RFC3339), nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", protocol.NewError(protocol.CodeBadRequest,
			"expiration must be an RFC3339 timestamp or a duration such as 24h or 90d")
	}
	if ts.Before(time.Now()) {
		return "", protocol.NewError(protocol.CodeBadRequest, "expiration must be in the future")
	}
	if ts.After(time.Now().Add(maxPreAuthKeyLifetime)) {
		return "", protocol.NewError(protocol.CodeBadRequest,
			"expiration must be at most %s away", maxPreAuthKeyLifetime)
	}
	return ts.UTC().Format(time.RFC3339), nil
}

// parseLifetime understands the day suffix Headscale's own CLI uses.
func parseLifetime(raw string) (time.Duration, error) {
	if strings.HasSuffix(raw, "d") {
		days, err := time.ParseDuration(strings.TrimSuffix(raw, "d") + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	return time.ParseDuration(raw)
}

func toPreAuthKey(k headscale.PreAuthKey, includeSecret bool) protocol.PreAuthKey {
	out := protocol.PreAuthKey{
		ID:         k.ID,
		Reusable:   k.Reusable,
		Ephemeral:  k.Ephemeral,
		Used:       k.Used,
		ACLTags:    k.ACLTags,
		Expiration: formatTimePtr(k.Expiration),
		CreatedAt:  formatTime(k.CreatedAt),
	}
	if k.User != nil {
		out.User = protocol.Ref{ID: k.User.ID, Name: k.User.Name}
	}
	if k.Expiration != nil {
		out.Expired = k.Expiration.Before(time.Now())
	}
	if includeSecret {
		out.Key = k.Key
	} else if k.Key != "" {
		// Listings must not carry usable key material.
		out.Key = maskKey(k.Key)
	}
	return out
}

func maskKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:12] + "-***"
}

// resolveUserID accepts either a numeric user id or a user name, because the
// Headscale API only accepts the former while operators only know the latter.
func (s *Service) resolveUserID(ctx context.Context, user string) (string, error) {
	if _, err := strconv.ParseUint(user, 10, 64); err == nil {
		return user, nil
	}
	users, err := s.hs.ListUsers(ctx)
	if err != nil {
		return "", headscaleError(err)
	}
	for _, u := range users {
		if u.Name == user {
			return u.ID, nil
		}
	}
	return "", protocol.NewError(protocol.CodeNotFound, "no Headscale user named %q", user)
}
