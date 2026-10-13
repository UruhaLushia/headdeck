package events

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	DefaultBuffer   = 256
	maxHistoryBytes = 8 << 20
)

type Event struct {
	ID         uint64
	Type       string
	Timestamp  time.Time
	ResourceID string
	Revision   uint64
	Data       json.RawMessage
}

type Hub struct {
	instance  string
	mu        sync.Mutex
	nextID    uint64
	ring      []Event
	ringSize  int
	ringBytes int
	subs      map[int]*subscription
	nextSub   int
	streams   int
	now       func() time.Time
}

type subscription struct {
	ch     chan Event
	closed bool
}

func NewHub() *Hub {
	return &Hub{
		instance: rand.Text(),
		ringSize: DefaultBuffer,
		subs:     map[int]*subscription{},
		now:      time.Now,
	}
}

func (h *Hub) Instance() string { return h.instance }

// Overflow closes the subscription so the consumer can explicitly resync.
func (h *Hub) Publish(eventType, resourceID string) Event {
	return h.publish(eventType, resourceID, nil)
}

func (h *Hub) PublishData(eventType, resourceID string, data any) (Event, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return Event{}, err
	}
	if len(payload) > 512<<10 {
		return Event{}, fmt.Errorf("event data exceeds 512 KiB")
	}
	return h.publish(eventType, resourceID, payload), nil
}

func (h *Hub) publish(eventType, resourceID string, data json.RawMessage) Event {
	h.mu.Lock()
	h.nextID++
	ev := Event{
		ID:         h.nextID,
		Type:       eventType,
		Timestamp:  h.now().UTC(),
		ResourceID: resourceID,
		Revision:   h.nextID,
		Data:       data,
	}
	h.retainLocked(ev)
	for id, sub := range h.subs {
		if sub.closed {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
			delete(h.subs, id)
			sub.close()
		}
	}
	h.mu.Unlock()
	return ev
}

func (h *Hub) Subscribe() (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subscribeLocked()
}

// SubscribeFrom makes replay and live subscription one atomic handoff.
func (h *Hub) SubscribeFrom(lastID uint64) ([]Event, <-chan Event, func(), uint64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	replay, ok := h.replayLocked(lastID)
	ch, cancel := h.subscribeLocked()
	return replay, ch, cancel, h.nextID, ok
}

func (h *Hub) subscribeLocked() (<-chan Event, func()) {
	id := h.nextSub
	h.nextSub++
	sub := &subscription{ch: make(chan Event, 32)}
	h.subs[id] = sub

	cancel := sync.OnceFunc(func() {
		h.mu.Lock()
		if !sub.closed {
			delete(h.subs, id)
			sub.close()
		}
		h.mu.Unlock()
	})
	return sub.ch, cancel
}

// Zero replays retained history; false requires a snapshot refresh.
func (h *Hub) Replay(lastID uint64) (events []Event, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.replayLocked(lastID)
}

func (h *Hub) replayLocked(lastID uint64) ([]Event, bool) {
	if lastID == 0 {
		return append([]Event(nil), h.ring...), true
	}
	if lastID > h.nextID {
		return nil, false
	}
	if len(h.ring) == 0 {
		return nil, lastID == h.nextID
	}
	oldest := h.ring[0].ID
	if lastID < oldest-1 {
		return nil, false
	}
	out := make([]Event, 0, len(h.ring))
	for _, ev := range h.ring {
		if ev.ID > lastID {
			out = append(out, ev)
		}
	}
	return out, true
}

func (h *Hub) LatestID() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.nextID
}

func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
