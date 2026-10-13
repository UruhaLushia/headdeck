package headscale

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) MoveNode(ctx context.Context, id, userID string) (*Node, error) {
	if _, err := parseID(id); err != nil {
		return nil, err
	}
	if _, err := parseID(userID); err != nil {
		return nil, err
	}
	if c.rpc != nil {
		return nil, &APIError{StatusCode: http.StatusNotImplemented, GRPCCode: 12, Message: "Headscale does not support changing a registered node's owner"}
	}
	var out struct {
		Node *Node `json:"node"`
	}
	body := struct {
		User string `json:"user"`
	}{userID}
	if err := c.do(ctx, http.MethodPost, "/api/v1/node/"+url.PathEscape(id)+"/user", nil, body, &out, false); err != nil {
		return nil, err
	}
	if out.Node == nil {
		return nil, &APIError{StatusCode: http.StatusInternalServerError, Message: "Headscale returned no node"}
	}
	return out.Node, nil
}
