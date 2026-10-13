package client

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (c *Client) ManagedConfig(ctx context.Context) (*protocol.ManagedConfig, error) {
	var out protocol.ManagedConfig
	err := c.do(ctx, http.MethodGet, protocol.PathManagedConfig, nil, &out)
	return &out, err
}

func (c *Client) SaveManagedConfig(ctx context.Context, req protocol.ManagedConfigUpdate) (*protocol.ManagedConfig, error) {
	var out protocol.ManagedConfig
	err := c.do(ctx, http.MethodPatch, protocol.PathManagedConfig, req, &out)
	return &out, err
}

func (c *Client) DiscardManagedConfig(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, protocol.PathManagedConfig, nil, nil)
}

func (c *Client) ApplyManagedConfig(ctx context.Context, revision string) (*protocol.Operation, error) {
	var out protocol.Operation
	err := c.do(ctx, http.MethodPost, protocol.PathManagedConfig+"/apply", protocol.DNSApply{Revision: revision}, &out)
	return &out, err
}

func (c *Client) Policy(ctx context.Context) (*protocol.PolicySettings, error) {
	var out protocol.PolicySettings
	err := c.do(ctx, http.MethodGet, protocol.PathPolicy, nil, &out)
	return &out, err
}

func (c *Client) SavePolicy(ctx context.Context, req protocol.PolicyUpdate) (*protocol.PolicySettings, error) {
	var out protocol.PolicySettings
	err := c.do(ctx, http.MethodPut, protocol.PathPolicy, req, &out)
	return &out, err
}

func (c *Client) CheckPolicy(ctx context.Context, value string) error {
	return c.do(ctx, http.MethodPost, protocol.PathPolicy+"/check", protocol.PolicyUpdate{Policy: value}, nil)
}

func (c *Client) DiscardPolicy(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, protocol.PathPolicy, nil, nil)
}

func (c *Client) ApplyPolicy(ctx context.Context, revision string) (*protocol.Operation, error) {
	var out protocol.Operation
	err := c.do(ctx, http.MethodPost, protocol.PathPolicy+"/apply", protocol.DNSApply{Revision: revision}, &out)
	return &out, err
}
