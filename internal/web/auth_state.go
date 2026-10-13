package web

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/go-webauthn/webauthn/webauthn"
)

type authState struct {
	kind, account, owner, secret string
	version                      uint64
	identity                     accounts.VerifiedIdentity
	ceremony                     *webauthn.SessionData
	expires                      time.Time
}
type authStates struct {
	mu    sync.Mutex
	items map[string]authState
}

func (f *authStates) put(state authState, now time.Time) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.items == nil {
		f.items = map[string]authState{}
	}
	for id, item := range f.items {
		if !item.expires.After(now) {
			delete(f.items, id)
		}
	}
	if len(f.items) >= 1024 {
		return "", false
	}
	id := rand.Text()
	state.expires = now.Add(5 * time.Minute)
	f.items[id] = state
	return id, true
}
func (f *authStates) take(id, kind, owner string, now time.Time) (authState, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.items[id]
	if !ok {
		return authState{}, false
	}
	if !item.expires.After(now) {
		delete(f.items, id)
		return authState{}, false
	}
	if item.kind != kind || item.owner != owner {
		return authState{}, false
	}
	delete(f.items, id)
	return item, true
}
