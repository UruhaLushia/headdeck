package service

import (
	"context"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) HeadscaleService(ctx context.Context) (*protocol.ServiceControl, error) {
	if s.control == nil || !s.control.Available() {
		return nil, protocol.NewError(protocol.CodeServiceCtlUnavailable, "Headscale service control is disabled or unavailable")
	}
	state, err := s.control.State(ctx)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeServiceCtlUnavailable, "%s", err)
	}
	return &protocol.ServiceControl{
		Supported: true,
		Driver:    s.control.Driver(),
		Unit:      s.control.Unit(),
		State:     state,
	}, nil
}
