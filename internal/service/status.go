package service

import (
	"context"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) Probe(ctx context.Context, force bool) protocol.HeadscaleStatus {
	const ttl = 10 * time.Second

	s.mu.Lock()
	if !force && s.snapshot != nil && time.Since(s.probedAt) < ttl {
		cached := *s.snapshot
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()

	status := s.probe(ctx)

	s.mu.Lock()
	s.snapshot = &status
	s.probedAt = time.Now()
	s.mu.Unlock()
	return status
}

func (s *Service) probe(ctx context.Context) protocol.HeadscaleStatus {
	out := protocol.HeadscaleStatus{
		BaseURL:   s.cfg.Headscale.BaseURL,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if s.cfg.Headscale.BaseURL == "" {
		out.LastError = protocol.NewError(protocol.CodeHeadscaleNotConfigured,
			"headscale.base_url is not configured")
		return out
	}
	out.Configured = true

	start := time.Now()
	if err := s.hs.Health(ctx); err != nil {
		out.LastError = headscaleError(err)
		return out
	}
	out.Reachable = true
	version, err := s.hs.Version(ctx)
	out.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		return out
	}
	out.Version = version.Version
	out.Commit = version.Commit

	key, err := s.hs.APIKeyInfo(ctx)
	if err != nil {
		out.LastError = headscaleError(err)
		return out
	}
	if key == nil {
		return out
	}
	out.APIKeyPrefix = key.Prefix
	out.APIKeyExpiry = key.Expiration
	if key.Expiration != nil {
		remaining := time.Until(*key.Expiration)
		if remaining <= 0 {
			out.KeyExpired = true
			out.KeyExpiresIn = "expired"
		} else {
			out.KeyExpiresIn = remaining.Truncate(time.Minute).String()
		}
	}
	return out
}

func (s *Service) Status(ctx context.Context, force bool) (*protocol.Status, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	hs := s.Probe(probeCtx, force)
	cancel()
	control := protocol.ServiceControl{Supported: false}
	if s.control != nil {
		control = protocol.ServiceControl{
			Supported: s.control.Available(),
			Driver:    s.control.Driver(),
			Unit:      s.control.Unit(),
			Message:   s.control.Message(),
		}
		if control.Supported {
			state, err := s.control.State(ctx)
			if err != nil {
				control.Message = err.Error()
			} else {
				control.State = state
			}
		}
	}
	return &protocol.Status{
		Headscale:      hs,
		ServiceControl: control,
	}, nil
}
