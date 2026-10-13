package headscale

import (
	"context"
	"net/http"
	"net/url"
	"time"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *Client) ListPreAuthKeys(ctx context.Context) ([]PreAuthKey, error) {
	var out struct {
		PreAuthKeys []PreAuthKey `json:"preAuthKeys"`
	}
	var err error
	if c.rpc != nil {
		err = c.call(ctx, "ListPreAuthKeys", &hsrpc.ListPreAuthKeysRequest{}, &hsrpc.ListPreAuthKeysResponse{}, &out, true)
	} else {
		err = c.get(ctx, "/api/v1/preauthkey", nil, &out)
	}
	if err != nil {
		return nil, err
	}
	return out.PreAuthKeys, nil
}

func (c *Client) CreatePreAuthKey(ctx context.Context, req CreatePreAuthKeyRequest) (*PreAuthKey, error) {
	var out struct {
		PreAuthKey *PreAuthKey `json:"preAuthKey"`
	}
	var err error
	if c.rpc != nil {
		user, parseErr := parseID(req.User)
		if parseErr != nil {
			return nil, parseErr
		}
		in := &hsrpc.CreatePreAuthKeyRequest{User: user, Reusable: req.Reusable, Ephemeral: req.Ephemeral, AclTags: req.ACLTags}
		if req.Expiration != "" {
			expiration, parseErr := time.Parse(time.RFC3339, req.Expiration)
			if parseErr != nil {
				return nil, parseErr
			}
			in.Expiration = timestamppb.New(expiration)
		}
		err = c.call(ctx, "CreatePreAuthKey", in, &hsrpc.CreatePreAuthKeyResponse{}, &out, false)
	} else {
		err = c.do(ctx, http.MethodPost, "/api/v1/preauthkey", nil, req, &out, false)
	}
	if err != nil {
		return nil, err
	}
	if out.PreAuthKey == nil {
		return nil, &APIError{StatusCode: http.StatusInternalServerError, Message: "headscale returned no pre-auth key"}
	}
	return out.PreAuthKey, nil
}

// DeletePreAuthKey takes the numeric id: the path form was removed in 0.23.
func (c *Client) DeletePreAuthKey(ctx context.Context, id string) error {
	numericID, err := parseID(id)
	if err != nil {
		return err
	}
	if c.rpc != nil {
		return c.call(ctx, "DeletePreAuthKey", &hsrpc.DeletePreAuthKeyRequest{Id: numericID}, &hsrpc.DeletePreAuthKeyResponse{}, nil, false)
	}
	query := url.Values{"id": []string{id}}
	return c.do(ctx, http.MethodDelete, "/api/v1/preauthkey", query, nil, nil, false)
}

func (c *Client) ExpirePreAuthKey(ctx context.Context, id string) error {
	numericID, err := parseID(id)
	if err != nil {
		return err
	}
	if c.rpc != nil {
		return c.call(ctx, "ExpirePreAuthKey", &hsrpc.ExpirePreAuthKeyRequest{Id: numericID}, &hsrpc.ExpirePreAuthKeyResponse{}, nil, false)
	}
	return c.do(ctx, http.MethodPost, "/api/v1/preauthkey/expire", nil,
		map[string]string{"id": id}, nil, false)
}
