package service

import (
	"context"
	"slices"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
)

func (s *Service) EditPolicy(ctx context.Context, revision string, item policyedit.Item, values map[string][]string, remove bool) (*protocol.PolicySettings, error) {
	current, err := s.Policy(ctx)
	if err != nil {
		return nil, err
	}
	if current.Revision != revision {
		return nil, protocol.NewError(protocol.CodeConflict, "policy changed; refresh before saving")
	}
	document, err := policyedit.Parse(current.Policy)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeBadRequest, "cannot edit policy: %v", err)
	}
	matched := false
	for _, candidate := range document.Items() {
		if item.Section == candidate.Section && item.Key == candidate.Key && item.Index == candidate.Index && item.Add == candidate.Add {
			item = candidate
			matched = true
			break
		}
	}
	if !matched {
		return nil, protocol.NewError(protocol.CodeBadRequest, "policy item is no longer available")
	}
	fields := document.Fields(item)
	if !remove {
		for name, value := range values {
			found := false
			for i := range fields {
				if fields[i].Name != name {
					continue
				}
				if !fields[i].Multi && len(value) != 1 {
					return nil, protocol.NewError(protocol.CodeBadRequest, "%s requires one value", name)
				}
				if len(fields[i].Choices) > 0 && !slices.Contains(fields[i].Choices, value[0]) {
					return nil, protocol.NewError(protocol.CodeBadRequest, "invalid option for %s", name)
				}
				fields[i].Values = value
				found = true
			}
			if !found {
				return nil, protocol.NewError(protocol.CodeBadRequest, "unknown policy field: %s", name)
			}
		}
	}
	value, err := document.Change(item, fields, remove)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeBadRequest, "%v", err)
	}
	return s.SavePolicy(ctx, protocol.PolicyUpdate{Policy: value, Revision: revision})
}
