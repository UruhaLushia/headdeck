package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type resourceSnapshot[T any] struct {
	mu       sync.Mutex
	items    []T
	revision uint64
}

func updateSnapshot[T any](state *resourceSnapshot[T], next []T, id func(T) string) *protocol.CollectionDiff[T] {
	if state.revision == 0 {
		state.items, state.revision = next, 1
		return nil
	}
	previous := make(map[string]T, len(state.items))
	for _, item := range state.items {
		previous[id(item)] = item
	}
	diff := &protocol.CollectionDiff[T]{BaseRevision: state.revision}
	for _, item := range next {
		key := id(item)
		if old, ok := previous[key]; !ok || !reflect.DeepEqual(old, item) {
			diff.Upsert = append(diff.Upsert, item)
		}
		delete(previous, key)
	}
	for _, item := range state.items {
		if _, ok := previous[id(item)]; ok {
			diff.Removed = append(diff.Removed, id(item))
		}
	}
	state.items = next
	if len(diff.Upsert)+len(diff.Removed) == 0 {
		return nil
	}
	state.revision++
	diff.Revision = state.revision
	return diff
}

func (s *Service) publishData(kind, id string, data any) {
	if _, err := s.hub.PublishData(kind, id, data); err != nil {
		s.publish(kind, id)
	}
}

func nodeID(n protocol.Node) string { return n.ID }
func userID(u protocol.User) string { return u.ID }

func (s *Service) observeNodes(nodes []protocol.Node) {
	if diff := updateSnapshot(&s.nodes, nodes, nodeID); diff != nil {
		s.publishData(protocol.EventNodesChanged, "", diff)
	}
}

func (s *Service) recordNode(node protocol.Node) {
	if s.nodes.revision == 0 {
		s.publish(protocol.EventNodesChanged, node.ID)
		return
	}
	diff := protocol.CollectionDiff[protocol.Node]{Upsert: []protocol.Node{node}}
	s.observeNodes(diff.Apply(s.nodes.items, nodeID))
}

func (s *Service) observeUsers(users []protocol.User) {
	if diff := updateSnapshot(&s.users, users, userID); diff != nil {
		s.publishData(protocol.EventUsersChanged, "", diff)
	}
}

func (s *Service) recordUser(user protocol.User) {
	if s.users.revision == 0 {
		s.publish(protocol.EventUsersChanged, user.ID)
		return
	}
	diff := protocol.CollectionDiff[protocol.User]{Upsert: []protocol.User{user}}
	s.observeUsers(diff.Apply(s.users.items, userID))
}

func (s *Service) watchResources() {
	defer s.watchWG.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.operationCtx.Done():
			return
		case <-ticker.C:
			if s.hub.Subscribers() == 0 || s.cfg.Headscale.BaseURL == "" || s.cfg.Headscale.APIKey == "" {
				continue
			}
			ctx, cancel := context.WithTimeout(s.operationCtx, 10*time.Second)
			if err := s.refreshResources(ctx); err != nil {
				cancel()
				continue
			}
			cancel()
		}
	}
}

func (s *Service) refreshResources(ctx context.Context) error {
	_, nodeErr := s.NodeSnapshot(ctx, "")
	_, userErr := s.UserSnapshot(ctx)
	return errors.Join(nodeErr, userErr)
}
