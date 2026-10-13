package events

import "sync"

const MaxStreams = 64

func (h *Hub) AcquireStream() (func(), bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.streams >= MaxStreams {
		return nil, false
	}
	h.streams++
	return sync.OnceFunc(func() {
		h.mu.Lock()
		h.streams--
		h.mu.Unlock()
	}), true
}

func eventBytes(ev Event) int {
	return len(ev.Data) + len(ev.Type) + len(ev.ResourceID)
}

func (h *Hub) retainLocked(ev Event) {
	size := eventBytes(ev)
	removed := 0
	for removed < len(h.ring) && (len(h.ring)-removed >= h.ringSize || h.ringBytes+size > maxHistoryBytes) {
		h.ringBytes -= eventBytes(h.ring[removed])
		removed++
	}
	if removed > 0 {
		remaining := copy(h.ring, h.ring[removed:])
		clear(h.ring[remaining:])
		h.ring = h.ring[:remaining]
	}
	if size <= maxHistoryBytes && h.ringSize > 0 {
		h.ring = append(h.ring, ev)
		h.ringBytes += size
	}
}

func (s *subscription) close() {
	s.closed = true
	close(s.ch)
	for range s.ch {
	}
}
