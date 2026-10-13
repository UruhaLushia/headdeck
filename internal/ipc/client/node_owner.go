package client

import (
	"context"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
	"net/url"
)

func (c *Client) MoveNode(ctx context.Context, id, userID string) (*protocol.Node, error) {
	var out protocol.Node
	if err := c.do(ctx, http.MethodPost, protocol.PathNodes+"/"+url.PathEscape(id)+"/user", protocol.MoveNodeRequest{UserID: userID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
