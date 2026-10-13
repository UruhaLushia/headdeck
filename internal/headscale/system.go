package headscale

import (
	"context"
	"strings"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
)

func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	var out VersionInfo
	if err := c.get(ctx, "/version", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Health(ctx context.Context) error {
	if c.rpc != nil {
		return c.call(ctx, "Health", &hsrpc.HealthRequest{}, &hsrpc.HealthResponse{}, nil, true)
	}
	var out struct {
		DatabaseConnectivity bool `json:"databaseConnectivity"`
	}
	return c.get(ctx, "/api/v1/health", nil, &out)
}

func (c *Client) APIKeyInfo(ctx context.Context) (*APIKey, error) {
	if c.rpc != nil {
		return nil, nil
	}
	key, err := c.APIKey()
	if err != nil {
		return nil, err
	}
	var out struct {
		APIKeys []APIKey `json:"apiKeys"`
	}
	if err := c.get(ctx, "/api/v1/apikey", nil, &out); err != nil {
		return nil, err
	}
	for i := range out.APIKeys {
		if keyMatchesPrefix(key, out.APIKeys[i].Prefix) {
			return &out.APIKeys[i], nil
		}
	}
	return nil, nil
}

func keyMatchesPrefix(key, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "***")
	if prefix == "" {
		return false
	}
	return strings.HasPrefix(key, prefix)
}
