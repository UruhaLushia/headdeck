package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) Routes(ctx context.Context) (*protocol.RouteList, error) {
	var out protocol.RouteList
	if err := c.do(ctx, http.MethodGet, protocol.PathRoutes, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) NodeRoutes(ctx context.Context, id string) (*protocol.RouteList, error) {
	var out protocol.RouteList
	path := protocol.PathNodes + "/" + url.PathEscape(id) + "/routes"
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetNodeRoutes(ctx context.Context, id string, routes []string) (*protocol.Node, error) {
	var out protocol.Node
	path := protocol.PathNodes + "/" + url.PathEscape(id) + "/routes"
	if err := c.do(ctx, http.MethodPost, path, protocol.SetRoutesRequest{Routes: routes}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetNodeTags(ctx context.Context, id string, tags []string) (*protocol.Node, error) {
	var out protocol.Node
	path := protocol.PathNodes + "/" + url.PathEscape(id) + "/tags"
	if err := c.do(ctx, http.MethodPost, path, protocol.SetTagsRequest{Tags: tags}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ExpireNode(ctx context.Context, id string) (*protocol.Node, error) {
	var out protocol.Node
	path := protocol.PathNodes + "/" + url.PathEscape(id) + "/expire"
	if err := c.do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
