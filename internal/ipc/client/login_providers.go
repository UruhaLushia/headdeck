package client

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
	"net/url"
)

func (c *Client) LoginProviders(ctx context.Context) ([]protocol.LoginProvider, error) {
	var out []protocol.LoginProvider
	err := c.do(ctx, http.MethodGet, protocol.PathLoginProviders, nil, &out)
	return out, err
}
func (c *Client) SaveLoginProvider(ctx context.Context, p protocol.LoginProvider) error {
	return c.do(ctx, http.MethodPut, protocol.PathLoginProviders+"/"+url.PathEscape(p.ID), p, nil)
}
func (c *Client) DeleteLoginProvider(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, protocol.PathLoginProviders+"/"+url.PathEscape(id), nil, nil)
}
