package service

import (
	"context"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) NodeSnapshot(ctx context.Context, user string) (*protocol.NodeList, error) {
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	nodes, err := s.hs.ListNodes(ctx, user)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := make([]protocol.Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toNode(n))
	}
	revision := uint64(0)
	if user == "" {
		s.observeNodes(out)
		revision = s.nodes.revision
	}
	return &protocol.NodeList{Nodes: out, Revision: revision}, nil
}

func (s *Service) Nodes(ctx context.Context, user string) ([]protocol.Node, error) {
	list, err := s.NodeSnapshot(ctx, user)
	if err != nil {
		return nil, err
	}
	return list.Nodes, nil
}

func (s *Service) UserSnapshot(ctx context.Context) (*protocol.UserList, error) {
	s.users.mu.Lock()
	defer s.users.mu.Unlock()
	return s.userSnapshot(ctx)
}

func (s *Service) userSnapshot(ctx context.Context) (*protocol.UserList, error) {
	users, err := s.hs.ListUsers(ctx)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := make([]protocol.User, 0, len(users))
	for _, u := range users {
		out = append(out, userDTO(u))
	}
	if s.accounts != nil {
		names := make(map[string]string, len(out))
		for _, u := range out {
			names[u.ID] = u.Name
		}
		if err := s.accounts.SyncHeadscaleUsers(ctx, names); err != nil {
			return nil, accountError(err)
		}
		if err := s.syncOIDCUsers(ctx, out); err != nil {
			return nil, err
		}
	}
	s.observeUsers(out)
	return &protocol.UserList{Users: out, Revision: s.users.revision}, nil
}

func (s *Service) Users(ctx context.Context) ([]protocol.User, error) {
	list, err := s.UserSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return list.Users, nil
}
