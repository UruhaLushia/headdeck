package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func connectionError(socket string, err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", protocol.NewError(protocol.CodeRequestTimeout,
			"timed out talking to the Headdeck daemon on %s", socket).WithRetryable(true), context.DeadlineExceeded)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return protocol.NewError(protocol.CodeRequestTimeout,
			"timed out talking to the Headdeck daemon on %s", socket).WithRetryable(true)
	}
	return protocol.NewError(protocol.CodeDaemonUnavailable,
		"cannot reach the Headdeck daemon on %s: %v", socket, err).WithRetryable(true)
}

func decodeError(status int, data []byte) error {
	var envelope protocol.ErrorResponse
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Error != nil {
		return &StatusError{StatusCode: status, Proto: envelope.Error}
	}
	msg := strings.TrimSpace(string(data))
	if msg == "" {
		msg = http.StatusText(status)
	}
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return &StatusError{
		StatusCode: status,
		Proto: &protocol.Error{
			Code:    codeForStatus(status),
			Message: msg,
		},
	}
}

func codeForStatus(status int) protocol.ErrorCode {
	switch status {
	case http.StatusUnauthorized:
		return protocol.CodeUnauthenticated
	case http.StatusForbidden:
		return protocol.CodePermissionDenied
	case http.StatusNotFound:
		return protocol.CodeNotFound
	case http.StatusUpgradeRequired:
		return protocol.CodeProtocolIncompatible
	case http.StatusTooManyRequests:
		return protocol.CodeRateLimited
	case http.StatusServiceUnavailable:
		return protocol.CodeDaemonUnavailable
	default:
		return protocol.CodeInternal
	}
}
