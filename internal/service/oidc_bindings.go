package service

import (
	"context"
	"fmt"
	"strconv"

	hi "github.com/UruhaLushia/headdeck/internal/headscale/identity"
)

func (s *Service) oidcBindings(ctx context.Context) ([]hi.Binding, error) {
	if s.accounts == nil {
		return nil, fmt.Errorf("account store is unavailable")
	}
	accounts, err := s.accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.hs.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]string, len(accounts))
	for _, a := range accounts {
		if a.HeadscaleUserID == "" {
			continue
		}
		byID[a.HeadscaleUserID] = a.ID
	}
	bindings := make([]hi.Binding, 0, len(byID))
	for _, user := range users {
		accountID, ok := byID[user.ID]
		if !ok {
			if user.ProviderID != "" || user.Provider == "oidc" {
				return nil, fmt.Errorf("bind Headscale user %s (%s) to a Headdeck account first", user.ID, user.Name)
			}
			continue
		}
		id, err := strconv.ParseUint(user.ID, 10, 64)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("invalid Headscale user ID")
		}
		bindings = append(bindings, hi.Binding{ID: id, Name: user.Name, Before: user.ProviderID, Provider: user.Provider, After: s.cfg.Server.Web.OIDCIssuer() + "/" + accountID})
	}
	if len(bindings) != len(byID) {
		return nil, fmt.Errorf("a bound Headscale user is missing")
	}
	return bindings, nil
}

func (s *Service) verifyOIDCBindings(ctx context.Context, bindings []hi.Binding) error {
	users, err := s.hs.ListUsers(ctx)
	if err != nil {
		return err
	}
	byID := make(map[uint64]hi.Binding, len(bindings))
	for _, b := range bindings {
		byID[b.ID] = b
	}
	found := 0
	for _, user := range users {
		id, err := strconv.ParseUint(user.ID, 10, 64)
		if err != nil {
			return err
		}
		b, ok := byID[id]
		if !ok {
			continue
		}
		if user.ProviderID != b.After || user.Provider != "oidc" || user.Name != b.Name {
			return fmt.Errorf("headscale identity %d did not switch", id)
		}
		found++
	}
	if found != len(bindings) {
		return fmt.Errorf("a bound Headscale user disappeared")
	}
	return nil
}
