package client

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type WatchOptions struct {
	// Resume after this ID; expired history triggers OnResync.
	LastEventID uint64
	// OnEvent receives every event. Returning an error stops the stream.
	OnEvent func(protocol.Event) error
	// Re-fetch a snapshot when replay cannot cover the gap.
	OnResync func(reason string)
	// OnCursor includes filtered events and resets after a daemon restart.
	OnCursor func(uint64)
}

// The caller owns reconnect and backoff policy.
func (c *Client) WatchEvents(ctx context.Context, opts WatchOptions) error {
	if err := c.ensureHandshake(ctx); err != nil {
		return err
	}
	defer c.ready.Store(false)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		localHost+protocol.PathEvents, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("X-Client", c.clientName)
	if opts.LastEventID > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatUint(opts.LastEventID, 10))
	}
	streamClient := *c.http
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return connectionError(c.socket, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return decodeError(resp.StatusCode, data)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		return protocol.NewError(protocol.CodeInternal,
			"daemon did not open an event stream (content type %q)", ct)
	}
	return c.readEvents(resp.Body, opts)
}
