package service

import (
	"context"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/headscale"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func userDTO(u headscale.User) protocol.User {
	return protocol.User{Provider: u.Provider, ID: u.ID, Name: u.Name, DisplayName: u.DisplayName, Email: u.Email,
		ProviderID: u.ProviderID, CreatedAt: formatTime(u.CreatedAt)}
}

func (s *Service) CreateUser(ctx context.Context, req protocol.CreateUserRequest) (*protocol.User, error) {
	req.Name = strings.TrimSpace(req.Name)
	if err := validateUserName(req.Name); err != nil {
		return nil, err
	}
	s.users.mu.Lock()
	defer s.users.mu.Unlock()
	u, err := s.hs.CreateUser(ctx, req.Name, req.DisplayName, req.Email)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := userDTO(*u)
	s.recordUser(out)
	return &out, nil
}

func (s *Service) RenameUser(ctx context.Context, id, name string) (*protocol.User, error) {
	name = strings.TrimSpace(name)
	if err := validateUserName(name); err != nil {
		return nil, err
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.users.mu.Lock()
	defer s.users.mu.Unlock()
	users, err := s.userSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range users.Users {
		if u.ID == id && u.Provider == "oidc" {
			return nil, protocol.NewError(protocol.CodeBadRequest, "OIDC-managed users cannot be renamed")
		}
	}
	if s.accounts != nil {
		linked, err := s.accounts.HeadscaleLinked(ctx, id)
		if err != nil {
			return nil, accountError(err)
		}
		if p, ok := principal.FromContext(ctx); ok && linked && !p.Has(principal.PermAccountsWrite) {
			return nil, protocol.NewError(protocol.CodePermissionDenied, "administrator required")
		}
		if err := s.accounts.CheckHeadscaleUsername(ctx, id, name); err != nil {
			return nil, accountError(err)
		}
	}
	u, err := s.hs.RenameUser(ctx, id, name)
	if err != nil {
		return nil, headscaleError(err)
	}
	out := userDTO(*u)
	if s.accounts != nil {
		list, err := s.accounts.List(ctx)
		if err != nil {
			return nil, accountError(err)
		}
		for _, a := range list {
			if a.HeadscaleUserID == id {
				if err := s.accounts.SetUsername(ctx, a.ID, out.Name); err != nil {
					return nil, accountError(err)
				}
			}
		}
	}
	s.recordUser(out)
	s.nodes.mu.Lock()
	defer s.nodes.mu.Unlock()
	if s.nodes.revision == 0 {
		s.publish(protocol.EventNodesChanged, "")
		return &out, nil
	}
	nodes := append([]protocol.Node(nil), s.nodes.items...)
	for i := range nodes {
		if nodes[i].User.ID == id {
			nodes[i].User.Name = out.Name
		}
	}
	s.observeNodes(nodes)
	return &out, nil
}

func validateUserName(name string) error {
	if name == "" || len(name) > 255 {
		return protocol.NewError(protocol.CodeBadRequest, "user name must contain 1 to 255 bytes")
	}
	return nil
}

func (s *Service) DeleteUser(ctx context.Context, id, confirm string) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.users.mu.Lock()
	defer s.users.mu.Unlock()
	users, err := s.userSnapshot(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, u := range users.Users {
		if u.ID == id {
			found = true
			if confirm == "" || confirm != u.Name {
				return protocol.NewError(protocol.CodeBadRequest, "user name confirmation does not match")
			}
		}
	}
	if !found {
		return protocol.NewError(protocol.CodeNotFound, "user not found")
	}
	if s.accounts != nil {
		linked, err := s.accounts.HeadscaleLinked(ctx, id)
		if err != nil {
			return err
		}
		if linked {
			return protocol.NewError(protocol.CodeBadRequest, "detach the Headdeck account before deleting this user")
		}
	}
	nodes, err := s.Nodes(ctx, "")
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if n.User.ID == id {
			return protocol.NewError(protocol.CodeBadRequest, "user still owns devices")
		}
	}
	if err := s.hs.DeleteUser(ctx, id); err != nil {
		return headscaleError(err)
	}
	diff := protocol.CollectionDiff[protocol.User]{Removed: []string{id}}
	s.observeUsers(diff.Apply(s.users.items, userID))
	return nil
}
