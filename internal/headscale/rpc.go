package headscale

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (c *Client) call(ctx context.Context, method string, in, response proto.Message, out any, read bool) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	attempts := 1
	if read {
		attempts += c.maxRetries
	}
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond):
			}
		}
		err = c.rpc.Invoke(ctx, "/headscale.v1.HeadscaleService/"+method, in, response)
		if err == nil {
			if out == nil {
				return nil
			}
			raw, marshalErr := (protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(response)
			if marshalErr != nil {
				return marshalErr
			}
			return json.Unmarshal(raw, out)
		}
		code := status.Code(err)
		if code != codes.Unavailable && code != codes.ResourceExhausted {
			break
		}
	}
	s := status.Convert(err)
	code := http.StatusInternalServerError
	switch s.Code() {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		code = http.StatusBadRequest
	case codes.Unauthenticated:
		code = http.StatusUnauthorized
	case codes.PermissionDenied:
		code = http.StatusForbidden
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		code = http.StatusConflict
	case codes.ResourceExhausted:
		code = http.StatusTooManyRequests
	case codes.Unimplemented:
		code = http.StatusNotImplemented
	case codes.Unavailable:
		code = http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		code = http.StatusGatewayTimeout
	case codes.Canceled:
		if err := ctx.Err(); err != nil {
			return err
		}
		code = 499
	}
	return &APIError{StatusCode: code, GRPCCode: int(s.Code()), Message: s.Message()}
}
