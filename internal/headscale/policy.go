package headscale

import (
	"context"
	"net/http"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
)

type Policy struct {
	Policy string `json:"policy"`
}

func (c *Client) Policy(ctx context.Context) (string, error) {
	var out Policy
	if c.rpc != nil {
		err := c.call(ctx, "GetPolicy", &hsrpc.GetPolicyRequest{}, &hsrpc.GetPolicyResponse{}, &out, true)
		return out.Policy, err
	}
	err := c.get(ctx, "/api/v1/policy", nil, &out)
	return out.Policy, err
}

func (c *Client) CheckPolicy(ctx context.Context, value string) error {
	if c.rpc != nil {
		return c.call(ctx, "CheckPolicy", &hsrpc.CheckPolicyRequest{Policy: value}, &hsrpc.CheckPolicyResponse{}, nil, false)
	}
	return c.do(ctx, http.MethodPost, "/api/v1/policy/check", nil, Policy{Policy: value}, nil, false)
}

func (c *Client) SetPolicy(ctx context.Context, value string) error {
	if c.rpc != nil {
		return c.call(ctx, "SetPolicy", &hsrpc.SetPolicyRequest{Policy: value}, &hsrpc.SetPolicyResponse{}, nil, false)
	}
	return c.do(ctx, http.MethodPut, "/api/v1/policy", nil, Policy{Policy: value}, nil, false)
}
