package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) Nodes(ctx context.Context, user string) (*protocol.NodeList, error) {
	path := protocol.PathNodes
	if user != "" {
		path += "?user=" + url.QueryEscape(user)
	}
	var out protocol.NodeList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Node(ctx context.Context, id string) (*protocol.Node, error) {
	var out protocol.Node
	if err := c.do(ctx, http.MethodGet, protocol.PathNodes+"/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Users(ctx context.Context) (*protocol.UserList, error) {
	var out protocol.UserList
	if err := c.do(ctx, http.MethodGet, protocol.PathUsers, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RenameNode(ctx context.Context, id, newName string) (*protocol.Node, error) {
	body := protocol.RenameNodeRequest{NewName: newName}
	var out protocol.Node
	path := protocol.PathNodes + "/" + url.PathEscape(id) + "/rename"
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteNode(ctx context.Context, id, confirm string) error {
	body := protocol.DeleteNodeRequest{ConfirmName: confirm}
	path := protocol.PathNodes + "/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, body, nil)
}
