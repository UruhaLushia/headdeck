package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRetentionAndSubscriptionCleanup(t *testing.T) {
	h := NewHub()
	for range 2000 {
		h.publish("nodes.changed", "n", make(json.RawMessage, 64<<10))
	}
	if h.ringBytes > maxHistoryBytes || len(h.ring) > DefaultBuffer || len(h.ring) == 0 {
		t.Fatalf("history size = %d events, %d bytes", len(h.ring), h.ringBytes)
	}
	if _, ok := h.Replay(1); ok {
		t.Fatal("byte eviction must invalidate old cursors")
	}
	h.ringSize = 1
	h.Publish("nodes.changed", "n")
	for _, ev := range h.ring[:cap(h.ring)][len(h.ring):] {
		if ev.Data != nil {
			t.Fatal("evicted payload remains in backing storage")
		}
	}
	for range 1000 {
		ch, cancel := h.Subscribe()
		h.Publish("nodes.changed", "n")
		cancel()
		cancel()
		if _, open := <-ch; open || h.Subscribers() != 0 {
			t.Fatal("cancel retained a subscription or queued data")
		}
	}
	releases := make([]func(), MaxStreams)
	for i := range releases {
		var ok bool
		releases[i], ok = h.AcquireStream()
		if !ok {
			t.Fatal("stream rejected before capacity")
		}
	}
	if _, ok := h.AcquireStream(); ok {
		t.Fatal("stream capacity was exceeded")
	}
	for _, release := range releases {
		release()
		release()
	}
	if h.streams != 0 {
		t.Fatal("stream slots were not released")
	}
}

func TestReplayGapIsDetected(t *testing.T) {
	h := NewHub()
	h.ringSize = 3
	for i := 0; i < 10; i++ {
		h.Publish("nodes.changed", "n")
	}
	if _, ok := h.Replay(1); ok {
		t.Fatal("a subscriber behind the retained history must be told to resync")
	}
	if _, ok := h.Replay(7); !ok {
		t.Fatal("a subscriber inside the retained history must be replayable")
	}
	events, ok := h.Replay(9)
	if !ok || len(events) != 1 || events[0].ID != 10 {
		t.Fatalf("replay from 9 = %+v (ok=%v)", events, ok)
	}
	// An id that was never issued cannot be replayed either.
	if _, ok := h.Replay(1 << 40); ok {
		t.Fatal("an impossible resume id must force a resync")
	}
	if events, ok := h.Replay(10); !ok || len(events) != 0 {
		t.Fatalf("replay at head = %+v (ok=%v)", events, ok)
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe()
	defer cancel()

	// Overflow must close the subscriber without blocking publishers.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			h.Publish("nodes.changed", "n")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow subscriber")
	}

	drained := 0
draining:
	for {
		select {
		case _, open := <-ch:
			if !open {
				break draining
			}
			drained++
			continue
		default:
		}
		break
	}
	if drained != 0 {
		t.Fatalf("overflow retained %d queued events", drained)
	}
	if h.Subscribers() != 0 {
		t.Fatal("overflowed subscription was not removed")
	}
}

func TestReplaySubscriptionHandoff(t *testing.T) {
	for range 100 {
		h := NewHub()
		start := make(chan struct{})
		done := make(chan struct{})
		go func() {
			<-start
			h.Publish("nodes.changed", "1")
			close(done)
		}()
		close(start)
		replay, ch, cancel, _, ok := h.SubscribeFrom(0)
		<-done
		if !ok {
			t.Fatal("fresh subscription cannot have a history gap")
		}
		if len(replay) == 0 {
			select {
			case ev := <-ch:
				if ev.ID != 1 {
					t.Fatalf("event = %+v", ev)
				}
			case <-time.After(time.Second):
				t.Fatal("event lost between replay and subscription")
			}
		} else if len(replay) != 1 || replay[0].ID != 1 {
			t.Fatalf("replay = %+v", replay)
		}
		select {
		case <-ch:
			t.Fatal("event duplicated across replay and subscription")
		default:
		}
		cancel()
	}
}
