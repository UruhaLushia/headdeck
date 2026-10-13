package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) PreAuthKeys(ctx context.Context, user string) (*protocol.PreAuthKeyList, error) {
	path := protocol.PathPreAuthKeys
	if user != "" {
		path += "?user=" + url.QueryEscape(user)
	}
	var out protocol.PreAuthKeyList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreatePreAuthKey(ctx context.Context, req protocol.CreatePreAuthKeyRequest) (*protocol.PreAuthKey, error) {
	var out protocol.PreAuthKey
	if err := c.do(ctx, http.MethodPost, protocol.PathPreAuthKeys, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeletePreAuthKey(ctx context.Context, id string) error {
	path := protocol.PathPreAuthKeys + "/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) ExpirePreAuthKey(ctx context.Context, id string) error {
	path := protocol.PathPreAuthKeys + "/" + url.PathEscape(id) + "/expire"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}
