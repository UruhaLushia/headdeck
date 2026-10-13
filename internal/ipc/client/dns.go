package client

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (c *Client) DNS(ctx context.Context) (*protocol.DNSSettings, error) {
	var out protocol.DNSSettings
	err := c.do(ctx, http.MethodGet, protocol.PathDNS, nil, &out)
	return &out, err
}
func (c *Client) SaveDNS(ctx context.Context, req protocol.DNSUpdate) (*protocol.DNSSettings, error) {
	var out protocol.DNSSettings
	err := c.do(ctx, http.MethodPut, protocol.PathDNS, req, &out)
	return &out, err
}
func (c *Client) DiscardDNS(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, protocol.PathDNS, nil, nil)
}
func (c *Client) ApplyDNS(ctx context.Context, revision string) (*protocol.Operation, error) {
	var out protocol.Operation
	err := c.do(ctx, http.MethodPost, protocol.PathDNS+"/apply", protocol.DNSApply{Revision: revision}, &out)
	return &out, err
}
