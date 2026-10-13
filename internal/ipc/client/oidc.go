package client

import (
	"context"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) ApplyOIDCTakeover(ctx context.Context, recover bool) (*protocol.Operation, error) {
	action := "apply"
	if recover {
		action = "recover"
	}
	var out protocol.Operation
	err := c.do(ctx, http.MethodPost, protocol.PathHeadscaleLogin+"/"+action, nil, &out)
	return &out, err
}
