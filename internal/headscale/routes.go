package headscale

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
)

func (c *Client) SetApprovedRoutes(ctx context.Context, id string, routes []string) (*Node, error) {
	numericID, err := parseID(id)
	if err != nil {
		return nil, err
	}
	if routes == nil {
		routes = []string{}
	}
	var out struct {
		Node *Node `json:"node"`
	}
	path := "/api/v1/node/" + url.PathEscape(id) + "/approve_routes"
	if c.rpc != nil {
		err = c.call(ctx, "SetApprovedRoutes", &hsrpc.SetApprovedRoutesRequest{NodeId: numericID, Routes: routes}, &hsrpc.SetApprovedRoutesResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, path, nil, map[string]any{"routes": routes}, &out, false)
	}
	if err != nil {
		return nil, err
	}
	return out.Node, nil
}

func (c *Client) SetTags(ctx context.Context, id string, tags []string) (*Node, error) {
	numericID, err := parseID(id)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	var out struct {
		Node *Node `json:"node"`
	}
	path := "/api/v1/node/" + url.PathEscape(id) + "/tags"
	if c.rpc != nil {
		err = c.call(ctx, "SetTags", &hsrpc.SetTagsRequest{NodeId: numericID, Tags: tags}, &hsrpc.SetTagsResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, path, nil, map[string]any{"tags": tags}, &out, false)
	}
	if err != nil {
		return nil, err
	}
	return out.Node, nil
}

func (c *Client) ExpireNode(ctx context.Context, id string) (*Node, error) {
	numericID, err := parseID(id)
	if err != nil {
		return nil, err
	}
	var out struct {
		Node *Node `json:"node"`
	}
	path := "/api/v1/node/" + url.PathEscape(id) + "/expire"
	if c.rpc != nil {
		err = c.call(ctx, "ExpireNode", &hsrpc.ExpireNodeRequest{NodeId: numericID}, &hsrpc.ExpireNodeResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, path, nil, nil, &out, false)
	}
	if err != nil {
		return nil, err
	}
	return out.Node, nil
}

func parseID(id string) (uint64, error) {
	if id == "" {
		return 0, &APIError{StatusCode: http.StatusBadRequest, Message: "resource id must not be empty"}
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, &APIError{
			StatusCode: http.StatusBadRequest,
			Message:    "resource id must be numeric, got " + strconv.Quote(id),
		}
	}
	return n, nil
}
