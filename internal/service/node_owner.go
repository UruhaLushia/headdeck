package service

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) MoveNode(ctx context.Context, id, userID string) (*protocol.Node, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	users, err := s.Users(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, u := range users {
		if u.ID == userID {
			found = true
			break
		}
	}
	if !found {
		return nil, protocol.NewError(protocol.CodeBadRequest, "owner user does not exist")
	}
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	n, err := s.hs.MoveNode(ctx, id, userID)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := toNode(*n)
	s.recordNode(out)
	return &out, nil
}
