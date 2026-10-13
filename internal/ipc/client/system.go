package client

import (
	"context"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) Ping(ctx context.Context) (*protocol.Ping, error) {
	var out protocol.Ping
	if err := c.do(ctx, http.MethodGet, protocol.PathPing, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Hello(ctx context.Context, req protocol.HelloRequest) (*protocol.HelloResponse, error) {
	var out protocol.HelloResponse
	if err := c.do(ctx, http.MethodPost, protocol.PathHello, req, &out); err != nil {
		return nil, err
	}
	if !(protocol.VersionRange{Min: req.ProtocolMin, Max: req.ProtocolMax}).Contains(out.ProtocolVersion) || !protocol.SupportedVersions.Contains(out.ProtocolVersion) {
		return nil, protocol.NewError(protocol.CodeProtocolIncompatible, "daemon selected unsupported protocol %d", out.ProtocolVersion)
	}
	c.ready.Store(true)
	return &out, nil
}

func (c *Client) ensureHandshake(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.ready.Load() {
		return nil
	}
	c.handshakeMu.Lock()
	defer c.handshakeMu.Unlock()
	if c.ready.Load() {
		return ctx.Err()
	}
	_, err := c.Hello(ctx, protocol.HelloRequest{
		Client: c.clientName, ProtocolMin: protocol.SupportedVersions.Min,
		ProtocolMax: protocol.SupportedVersions.Max, Features: protocol.AllFeatures,
	})
	return err
}

func (c *Client) Status(ctx context.Context, refresh bool) (*protocol.Status, error) {
	path := protocol.PathStatus
	if refresh {
		path += "?refresh=1"
	}
	var out protocol.Status
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
