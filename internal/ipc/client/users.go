package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) CreateUser(ctx context.Context, req protocol.CreateUserRequest) (*protocol.User, error) {
	var out protocol.User
	if err := c.do(ctx, http.MethodPost, protocol.PathUsers, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Client) RenameUser(ctx context.Context, id, name string) (*protocol.User, error) {
	var out protocol.User
	if err := c.do(ctx, http.MethodPost, protocol.PathUsers+"/"+url.PathEscape(id)+"/rename", protocol.RenameUserRequest{NewName: name}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c *Client) DeleteUser(ctx context.Context, id, confirm string) error {
	return c.do(ctx, http.MethodDelete, protocol.PathUsers+"/"+url.PathEscape(id), protocol.DeleteUserRequest{ConfirmName: confirm}, nil)
}
