package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) Close() {
	if c == nil {
		return
	}
	c.http.CloseIdleConnections()
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if path != protocol.PathPing && path != protocol.PathHello {
		if err := c.ensureHandshake(ctx); err != nil {
			return err
		}
	}
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, localHost+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	req.Header.Set("X-Client", c.clientName)
	req.Header.Set("X-Request-Id", newRequestID())
	resp, err := c.http.Do(req)
	if err != nil {
		c.ready.Store(false)
		return connectionError(c.socket, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		c.ready.Store(false)
		return connectionError(c.socket, err)
	}
	if len(data) > maxResponseBytes {
		return protocol.NewError(protocol.CodePayloadTooLarge, "daemon response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return protocol.NewError(protocol.CodeInternal, "daemon returned an unexpected redirect")
	}
	if resp.StatusCode >= 400 {
		return decodeError(resp.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if len(data) == 0 {
		return protocol.NewError(protocol.CodeInternal, "daemon returned an empty response body")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return protocol.NewError(protocol.CodeInternal,
			"cannot decode daemon response: %v", err)
	}
	return nil
}
