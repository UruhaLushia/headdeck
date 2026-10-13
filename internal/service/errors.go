package service

import (
	"errors"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/headscale"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func headscaleError(err error) *protocol.Error {
	var apiErr *headscale.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusNotImplemented:
			return protocol.NewError(protocol.CodeUnsupported, "%s", apiErr.Summary()).WithRetryable(false)
		case apiErr.Unauthorized():
			return protocol.NewError(protocol.CodeHeadscaleUnauthorized,
				"Headscale rejected the API key").WithRetryable(false)
		case apiErr.NotFound():
			return protocol.NewError(protocol.CodeNotFound, "Headscale reported not found")
		default:
			return protocol.NewError(protocol.CodeHeadscaleUnavailable,
				"Headscale API error: %s", apiErr.Summary()).WithRetryable(apiErr.Retryable())
		}
	}
	var netErr *headscale.TransportError
	if errors.As(err, &netErr) {
		return protocol.NewError(protocol.CodeHeadscaleUnavailable,
			"cannot reach the Headscale API: %v", netErr.Err).WithRetryable(true)
	}
	return protocol.NewError(protocol.CodeHeadscaleUnavailable,
		"Headscale request failed: %v", err).WithRetryable(true)
}
