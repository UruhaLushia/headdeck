package service

import (
	"context"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) Node(ctx context.Context, id string) (*protocol.Node, error) {
	n, err := s.hs.GetNode(ctx, id)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := toNode(*n)
	return &out, nil
}

func (s *Service) RegisterNode(ctx context.Context, req protocol.RegisterNodeRequest) (*protocol.Node, error) {
	key := strings.TrimSpace(req.Key)
	if key == "" || len(key) > 256 || strings.ContainsAny(key, " \t\r\n/?#") {
		return nil, protocol.NewError(protocol.CodeBadRequest, "a valid node registration key is required")
	}
	userID := strings.TrimSpace(req.UserID)
	if p, ok := principal.FromContext(ctx); ok && userID == "" {
		userID = p.HeadscaleUserID
	}
	if err := requireOwner(ctx, userID); err != nil {
		return nil, err
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	users, err := s.Users(ctx)
	if err != nil {
		return nil, err
	}
	var name string
	for _, user := range users {
		if user.ID == userID {
			name = user.Name
			break
		}
	}
	if name == "" {
		return nil, protocol.NewError(protocol.CodeBadRequest, "owner user does not exist")
	}
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	node, err := s.hs.RegisterNode(ctx, name, key)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := toNode(*node)
	s.recordNode(out)
	return &out, nil
}

func (s *Service) RenameNode(ctx context.Context, id, newName string) (*protocol.Node, error) {
	name := strings.TrimSpace(newName)
	if err := validateNodeName(name); err != nil {
		return nil, err
	}
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	if err := s.requireNodeOwner(ctx, id); err != nil {
		return nil, err
	}
	n, err := s.hs.RenameNode(ctx, id, name)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := toNode(*n)
	s.recordNode(out)
	return &out, nil
}

func (s *Service) DeleteNode(ctx context.Context, id, confirmName string) error {
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	node, err := s.hs.GetNode(ctx, id)
	if err != nil {
		return headscaleError(err)
	}
	if confirmName != "" && confirmName != node.GivenName && confirmName != node.Name {
		return protocol.NewError(protocol.CodeBadRequest,
			"confirmation %q does not match node %q", confirmName, node.GivenName)
	}
	if err := s.hs.DeleteNode(ctx, id); err != nil {
		return headscaleError(err)
	}
	if s.nodes.revision == 0 {
		s.publish(protocol.EventNodesChanged, id)
	} else {
		diff := protocol.CollectionDiff[protocol.Node]{Removed: []string{id}}
		s.observeNodes(diff.Apply(s.nodes.items, nodeID))
	}
	return nil
}

func validateNodeName(name string) error {
	if name == "" {
		return protocol.NewError(protocol.CodeBadRequest, "node name must not be empty")
	}
	if len(name) > 63 {
		return protocol.NewError(protocol.CodeBadRequest, "node name must be at most 63 characters")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' && i != 0 && i != len(name)-1:
		default:
			return protocol.NewError(protocol.CodeBadRequest,
				"node name may only contain lowercase letters, digits and inner hyphens")
		}
	}
	return nil
}
