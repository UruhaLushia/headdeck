package client

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func TestEventInstanceChangeWithOverlappingIDs(t *testing.T) {
	c := &Client{}
	resyncs, received := 0, 0
	opts := WatchOptions{
		OnResync: func(reason string) {
			if reason != "daemon_restarted" {
				t.Fatal(reason)
			}
			resyncs++
		},
		OnEvent: func(ev protocol.Event) error {
			if ev.Type != protocol.EventNodesChanged || len(ev.Data) == 0 {
				t.Fatal("diff data was lost", ev)
			}
			received++
			return nil
		},
	}
	for _, instance := range []string{"first", "first", "second"} {
		frames := "event: hello\ndata: {\"instance\":\"" + instance + "\"}\n\n" +
			"event: nodes.changed\nid: 1\ndata: {\"type\":\"nodes.changed\",\"data\":{\"base_revision\":1,\"revision\":2,\"removed\":[\"1\"]}}\n\n"
		if err := c.readEvents(strings.NewReader(frames), opts); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
	}
	if resyncs != 1 || received != 3 {
		t.Fatalf("resyncs=%d, events=%d", resyncs, received)
	}
}
