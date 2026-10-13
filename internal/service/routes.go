package service

import (
	"context"
	"net"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/headscale"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Service) Routes(ctx context.Context) ([]protocol.Route, error) {
	nodes, err := s.hs.ListNodes(ctx, "")
	if err != nil {
		return nil, headscaleError(err)
	}
	var out []protocol.Route
	for _, n := range nodes {
		approved := make(map[string]bool, len(n.ApprovedRoutes))
		for _, r := range n.ApprovedRoutes {
			approved[r] = true
		}
		enabled := make(map[string]bool, len(n.SubnetRoutes))
		for _, r := range n.SubnetRoutes {
			enabled[r] = true
		}
		seen := map[string]bool{}
		add := func(prefix string, available bool) {
			if prefix == "" || seen[prefix] {
				return
			}
			seen[prefix] = true
			route := protocol.Route{
				NodeID:    n.ID,
				NodeName:  nodeDisplayName(n),
				Prefix:    prefix,
				Available: available,
				Approved:  approved[prefix],
				Enabled:   enabled[prefix],
			}
			if n.User != nil {
				route.User = protocol.Ref{ID: n.User.ID, Name: n.User.Name}
			}
			out = append(out, route)
		}
		for _, r := range n.AvailableRoutes {
			add(r, true)
		}
		for _, r := range n.ApprovedRoutes {
			add(r, false)
		}
	}
	return out, nil
}

func (s *Service) NodeRoutes(ctx context.Context, id string) ([]protocol.Route, error) {
	node, err := s.hs.GetNode(ctx, id)
	if err != nil {
		return nil, headscaleError(err)
	}
	all, err := s.Routes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.Route, 0, len(all))
	for _, r := range all {
		if r.NodeID == node.ID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Service) SetNodeRoutes(ctx context.Context, id string, routes []string) (*protocol.Node, error) {
	cleaned := make([]string, 0, len(routes))
	seen := map[string]bool{}
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(r); err != nil {
			return nil, protocol.NewError(protocol.CodeBadRequest,
				"route %q is not a CIDR prefix", r)
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		cleaned = append(cleaned, r)
	}
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	node, err := s.hs.SetApprovedRoutes(ctx, id, cleaned)
	if err != nil {
		return nil, headscaleError(err)
	}
	if node == nil {
		return nil, protocol.NewError(protocol.CodeInternal, "Headscale returned no node")
	}
	s.publish(protocol.EventRoutesChanged, id)
	out := toNode(*node)
	s.recordNode(out)
	return &out, nil
}

func (s *Service) SetNodeTags(ctx context.Context, id string, tags []string) (*protocol.Node, error) {
	cleaned := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "tag:") || len(t) <= len("tag:") {
			return nil, protocol.NewError(protocol.CodeBadRequest,
				"tag %q must look like tag:name", t)
		}
		cleaned = append(cleaned, t)
	}
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	node, err := s.hs.SetTags(ctx, id, cleaned)
	if err != nil {
		return nil, headscaleError(err)
	}
	if node == nil {
		return nil, protocol.NewError(protocol.CodeInternal, "Headscale returned no node")
	}
	out := toNode(*node)
	s.recordNode(out)
	return &out, nil
}

func (s *Service) ExpireNode(ctx context.Context, id string) (*protocol.Node, error) {
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	if err := s.requireNodeOwner(ctx, id); err != nil {
		return nil, err
	}
	node, err := s.hs.ExpireNode(ctx, id)
	if err != nil {
		return nil, headscaleError(err)
	}
	if node == nil {
		// Older Headscale versions answer with an empty body.
		fresh, ferr := s.hs.GetNode(ctx, id)
		if ferr != nil {
			s.publish(protocol.EventNodesChanged, id)
			return nil, headscaleError(ferr)
		}
		out := toNode(*fresh)
		s.recordNode(out)
		return &out, nil
	}
	out := toNode(*node)
	s.recordNode(out)
	return &out, nil
}

func nodeDisplayName(n headscale.Node) string {
	if n.GivenName != "" {
		return n.GivenName
	}
	return n.Name
}
