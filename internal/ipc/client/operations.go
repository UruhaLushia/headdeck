package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type OperationList struct {
	Operations []*protocol.Operation `json:"operations"`
}

func (c *Client) Operations(ctx context.Context) (*OperationList, error) {
	var out OperationList
	if err := c.do(ctx, http.MethodGet, protocol.PathOperations, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Operation(ctx context.Context, id string) (*protocol.Operation, error) {
	var out protocol.Operation
	path := protocol.PathOperations + "/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) HeadscaleAction(ctx context.Context, action protocol.ServiceAction, reason string) (*protocol.Operation, error) {
	if !action.Valid() {
		return nil, protocol.NewError(protocol.CodeBadRequest, "unknown service action")
	}
	var out protocol.Operation
	if err := c.do(ctx, http.MethodPost, protocol.PathHeadscaleService+"/"+string(action), protocol.ServiceActionRequest{Reason: reason}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) HeadscaleService(ctx context.Context) (*protocol.ServiceControl, error) {
	var out protocol.ServiceControl
	if err := c.do(ctx, http.MethodGet, protocol.PathHeadscaleService, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
