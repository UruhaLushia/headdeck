package headscale

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var ErrNoAPIKey = errors.New("no Headscale API key available")

type APIError struct {
	StatusCode int
	// GRPCCode is set when the response body used the grpc-gateway shape
	// {"code":5,...}; it is not an HTTP status.
	GRPCCode int
	Message  string
	RFC7807  bool
}

func (e *APIError) Error() string {
	return fmt.Sprintf("headscale API %d: %s", e.StatusCode, e.Message)
}

func (e *APIError) Unauthorized() bool { return e.StatusCode == http.StatusUnauthorized }

func (e *APIError) Retryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == http.StatusTooManyRequests
}

// NotFound covers both the explicit 404 and the grpc-gateway habit of mapping
// unmapped domain errors to 500 with gRPC code 2 on Headscale <= 0.29.
func (e *APIError) NotFound() bool {
	if e.StatusCode == http.StatusNotFound {
		return true
	}
	return e.StatusCode == http.StatusInternalServerError && e.GRPCCode == 2 &&
		strings.Contains(strings.ToLower(e.Message), "not found")
}

func (e *APIError) Summary() string {
	if e.Message == "" {
		return http.StatusText(e.StatusCode)
	}
	return e.Message
}

type TransportError struct {
	Err error
}

func (e *TransportError) Error() string { return "headscale transport: " + e.Err.Error() }

func (e *TransportError) Unwrap() error { return e.Err }
