package client

import (
	"context"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) Settings(ctx context.Context) (*protocol.Settings, error) {
	var out protocol.Settings
	err := c.do(ctx, http.MethodGet, protocol.PathSettings, nil, &out)
	return &out, err
}

func (c *Client) UpdateSettings(ctx context.Context, changes map[string]string) (*protocol.Settings, error) {
	var out protocol.Settings
	err := c.do(ctx, http.MethodPatch, protocol.PathSettings, protocol.SettingsUpdate{Changes: changes}, &out)
	return &out, err
}
