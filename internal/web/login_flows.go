package web

import (
	"crypto/rand"
	"crypto/subtle"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

type loginFlow struct {
	state, secret, nonce, verifier string
	provider                       string
	expires                        time.Time
	account, session               string
	version                        uint64
}

func challenge(verifier string) string { return oauth2.S256ChallengeFromVerifier(verifier) }
func (f *loginFlows) bind(state, account, session string, version uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	flow, ok := f.items[state]
	if !ok {
		return
	}
	flow.account = account
	flow.session = session
	flow.version = version
	f.items[state] = flow
}

type loginFlows struct {
	mu    sync.Mutex
	items map[string]loginFlow
}

func (f *loginFlows) begin(provider string, now time.Time) (loginFlow, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.items == nil {
		f.items = make(map[string]loginFlow)
	}
	for key, flow := range f.items {
		if !flow.expires.After(now) {
			delete(f.items, key)
		}
	}
	if len(f.items) >= 1024 {
		return loginFlow{}, false
	}
	flow := loginFlow{provider: provider, state: rand.Text(), secret: rand.Text(), nonce: rand.Text(), verifier: oauth2.GenerateVerifier(), expires: now.Add(5 * time.Minute)}
	f.items[flow.state] = flow
	return flow, true
}

func (f *loginFlows) take(state, secret, provider string, now time.Time) (loginFlow, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	flow, ok := f.items[state]
	if !ok || flow.provider != provider {
		return loginFlow{}, false
	}
	if !flow.expires.After(now) {
		delete(f.items, state)
		return loginFlow{}, false
	}
	if subtle.ConstantTimeCompare([]byte(flow.secret), []byte(secret)) != 1 {
		return loginFlow{}, false
	}
	delete(f.items, state)
	return flow, true
}
