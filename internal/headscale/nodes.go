package headscale

import (
	"context"
	"net/http"
	"net/url"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
)

func (c *Client) ListNodes(ctx context.Context, user string) ([]Node, error) {
	query := url.Values{}
	if user != "" {
		query.Set("user", user)
	}
	var out struct {
		Nodes []Node `json:"nodes"`
	}
	var err error
	if c.rpc != nil {
		err = c.call(ctx, "ListNodes", &hsrpc.ListNodesRequest{User: user}, &hsrpc.ListNodesResponse{}, &out, true)
	} else {
		err = c.get(ctx, "/api/v1/node", query, &out)
	}
	if err != nil {
		return nil, err
	}
	return out.Nodes, nil
}

func (c *Client) GetNode(ctx context.Context, id string) (*Node, error) {
	numericID, err := parseID(id)
	if err != nil {
		return nil, err
	}
	var out struct {
		Node *Node `json:"node"`
	}
	if c.rpc != nil {
		err = c.call(ctx, "GetNode", &hsrpc.GetNodeRequest{NodeId: numericID}, &hsrpc.GetNodeResponse{}, &out, true)
	} else {
		err = c.get(ctx, "/api/v1/node/"+url.PathEscape(id), nil, &out)
	}
	if err != nil {
		return nil, err
	}
	if out.Node == nil {
		return nil, &APIError{StatusCode: http.StatusNotFound, Message: "node not found"}
	}
	return out.Node, nil
}

func (c *Client) RegisterNode(ctx context.Context, user, key string) (*Node, error) {
	var out struct {
		Node *Node `json:"node"`
	}
	in := &hsrpc.RegisterNodeRequest{User: user, Key: key}
	var err error
	if c.rpc != nil {
		err = c.call(ctx, "RegisterNode", in, &hsrpc.RegisterNodeResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, "/api/v1/node/register", nil, in, &out, false)
	}
	if err != nil {
		return nil, err
	}
	if out.Node == nil {
		return nil, &APIError{StatusCode: http.StatusBadGateway, Message: "Headscale returned no registered node"}
	}
	return out.Node, nil
}

func (c *Client) RenameNode(ctx context.Context, id, newName string) (*Node, error) {
	numericID, err := parseID(id)
	if err != nil {
		return nil, err
	}
	path := "/api/v1/node/" + url.PathEscape(id) + "/rename/" + url.PathEscape(newName)
	var out struct {
		Node *Node `json:"node"`
	}
	if c.rpc != nil {
		err = c.call(ctx, "RenameNode", &hsrpc.RenameNodeRequest{NodeId: numericID, NewName: newName}, &hsrpc.RenameNodeResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, path, nil, nil, &out, false)
	}
	if err != nil {
		return nil, err
	}
	if out.Node == nil {
		return nil, &APIError{StatusCode: http.StatusInternalServerError, Message: "headscale returned no node"}
	}
	return out.Node, nil
}

func (c *Client) DeleteNode(ctx context.Context, id string) error {
	numericID, err := parseID(id)
	if err != nil {
		return err
	}
	if c.rpc != nil {
		return c.call(ctx, "DeleteNode", &hsrpc.DeleteNodeRequest{NodeId: numericID}, &hsrpc.DeleteNodeResponse{}, nil, false)
	}
	return c.do(ctx, http.MethodDelete, "/api/v1/node/"+url.PathEscape(id), nil, nil, nil, false)
}

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var out struct {
		Users []User `json:"users"`
	}
	var err error
	if c.rpc != nil {
		err = c.call(ctx, "ListUsers", &hsrpc.ListUsersRequest{}, &hsrpc.ListUsersResponse{}, &out, true)
	} else {
		err = c.get(ctx, "/api/v1/user", nil, &out)
	}
	if err != nil {
		return nil, err
	}
	if out.Users == nil {
		return nil, &APIError{StatusCode: http.StatusBadGateway, Message: "Headscale returned no user list"}
	}
	seen := make(map[string]bool, len(out.Users))
	for _, user := range out.Users {
		if user.ID == "" || seen[user.ID] {
			return nil, &APIError{StatusCode: http.StatusBadGateway, Message: "Headscale returned invalid user IDs"}
		}
		seen[user.ID] = true
	}
	return out.Users, nil
}
