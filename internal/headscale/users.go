package headscale

import (
	"context"
	"net/http"
	"net/url"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
)

func (c *Client) CreateUser(ctx context.Context, name, displayName, email string) (*User, error) {
	body := struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
	}{name, displayName, email}
	var out struct {
		User *User `json:"user"`
	}
	var err error
	if c.rpc != nil {
		err = c.call(ctx, "CreateUser", &hsrpc.CreateUserRequest{Name: name, DisplayName: displayName, Email: email}, &hsrpc.CreateUserResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, "/api/v1/user", nil, body, &out, false)
	}
	if err != nil {
		return nil, err
	}
	if out.User == nil {
		return nil, &APIError{StatusCode: http.StatusInternalServerError, Message: "headscale returned no user"}
	}
	return out.User, nil
}

func (c *Client) RenameUser(ctx context.Context, id, name string) (*User, error) {
	numericID, err := parseID(id)
	if err != nil {
		return nil, err
	}
	var out struct {
		User *User `json:"user"`
	}
	if c.rpc != nil {
		err = c.call(ctx, "RenameUser", &hsrpc.RenameUserRequest{OldId: numericID, NewName: name}, &hsrpc.RenameUserResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, "/api/v1/user/"+url.PathEscape(id)+"/rename/"+url.PathEscape(name), nil, nil, &out, false)
	}
	if err != nil {
		return nil, err
	}
	if out.User == nil {
		return nil, &APIError{StatusCode: http.StatusInternalServerError, Message: "Headscale returned no user"}
	}
	return out.User, nil
}

func (c *Client) DeleteUser(ctx context.Context, id string) error {
	numericID, err := parseID(id)
	if err != nil {
		return err
	}
	if c.rpc != nil {
		return c.call(ctx, "DeleteUser", &hsrpc.DeleteUserRequest{Id: numericID}, &hsrpc.DeleteUserResponse{}, nil, false)
	}
	return c.do(ctx, http.MethodDelete, "/api/v1/user/"+url.PathEscape(id), nil, nil, nil, false)
}
