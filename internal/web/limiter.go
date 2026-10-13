package web

import (
	"sync"
	"time"
)

const maxLoginClients = 4096

type attemptLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string][]time.Time
}

func newAttemptLimiter(limit int, window time.Duration) *attemptLimiter {
	return &attemptLimiter{limit: limit, window: window, attempts: map[string][]time.Time{}}
}

func (l *attemptLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	for client, attempts := range l.attempts {
		if len(attempts) == 0 || !attempts[len(attempts)-1].After(cutoff) {
			delete(l.attempts, client)
		}
	}
	if _, exists := l.attempts[key]; !exists && len(l.attempts) >= maxLoginClients {
		return false
	}
	kept := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.attempts[key] = kept
		return false
	}
	l.attempts[key] = append(kept, now)
	return true
}
