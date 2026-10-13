package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
)

type bodyKey struct{}

func withBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, bodyKey{}, body)
}

func bodyFrom(ctx context.Context) []byte {
	body, _ := ctx.Value(bodyKey{}).([]byte)
	return body
}

func requirePermission(p *principal.Principal, perm principal.Permission) *protocol.Error {
	if p == nil {
		return protocol.NewError(protocol.CodeUnauthenticated, "no authenticated principal")
	}
	if !p.Has(perm) {
		return protocol.NewError(protocol.CodePermissionDenied,
			"role %s is not allowed to perform %s", p.Role, perm)
	}
	return nil
}

func decodeBody(r *http.Request, max int64) ([]byte, *protocol.Error) {
	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, max))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, protocol.NewError(protocol.CodePayloadTooLarge,
				"request body exceeds %d bytes", max)
		}
		return nil, protocol.NewError(protocol.CodeBadRequest, "cannot read request body")
	}
	return body, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(ctx context.Context, w http.ResponseWriter, err *protocol.Error) {
	if err == nil {
		return
	}
	if err.RequestID == "" {
		err.RequestID = log.RequestID(ctx)
	}
	writeJSON(w, protocol.StatusForCode(err.Code), protocol.ErrorResponse{Error: err})
}

func decodeJSON(data []byte, out any) *protocol.Error {
	if len(data) == 0 {
		return protocol.NewError(protocol.CodeBadRequest, "request body is required")
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return protocol.NewError(protocol.CodeBadRequest, "invalid request body: %s", safeDecodeError(err))
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return protocol.NewError(protocol.CodeBadRequest, "request body must contain exactly one JSON value")
	}
	return nil
}

func safeDecodeError(err error) string {
	msg := err.Error()
	if len(msg) > 160 {
		msg = msg[:160] + "..."
	}
	return msg
}
