package protocol

import (
	"errors"
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	CodeBadRequest             ErrorCode = "BAD_REQUEST"
	CodeUnauthenticated        ErrorCode = "UNAUTHENTICATED"
	CodePermissionDenied       ErrorCode = "PERMISSION_DENIED"
	CodeProtocolIncompatible   ErrorCode = "PROTOCOL_INCOMPATIBLE"
	CodeNotFound               ErrorCode = "NOT_FOUND"
	CodeConflict               ErrorCode = "CONFLICT"
	CodeOperationInProgress    ErrorCode = "OPERATION_IN_PROGRESS"
	CodeUnsupported            ErrorCode = "UNSUPPORTED"
	CodeHeadscaleUnavailable   ErrorCode = "HEADSCALE_UNAVAILABLE"
	CodeHeadscaleUnauthorized  ErrorCode = "HEADSCALE_UNAUTHORIZED"
	CodeHeadscaleVersionBad    ErrorCode = "HEADSCALE_VERSION_UNSUPPORTED"
	CodeServiceCtlUnavailable  ErrorCode = "SERVICE_CONTROL_UNAVAILABLE"
	CodeClientConfig           ErrorCode = "CLIENT_CONFIG_ERROR"
	CodeDaemonUnavailable      ErrorCode = "DAEMON_UNAVAILABLE"
	CodeDaemonNotRunning       ErrorCode = "DAEMON_NOT_RUNNING"
	CodeInternal               ErrorCode = "INTERNAL"
	CodePayloadTooLarge        ErrorCode = "PAYLOAD_TOO_LARGE"
	CodeRateLimited            ErrorCode = "RATE_LIMITED"
	CodeServiceAlreadyStarted  ErrorCode = "SERVICE_ALREADY_STARTED"
	CodeServiceNotInstalled    ErrorCode = "SERVICE_NOT_INSTALLED"
	CodeMethodNotAllowed       ErrorCode = "METHOD_NOT_ALLOWED"
	CodeRequestTimeout         ErrorCode = "REQUEST_TIMEOUT"
	CodeDependencyUnavailable  ErrorCode = "DEPENDENCY_UNAVAILABLE"
	CodePeerNotAuthorized      ErrorCode = "PEER_NOT_AUTHORIZED"
	CodeHeadscaleNotConfigured ErrorCode = "HEADSCALE_NOT_CONFIGURED"
)

// Never expose upstream bodies that might contain credentials.
type Error struct {
	Code      ErrorCode         `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id,omitempty"`
	Retryable bool              `json:"retryable,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error *Error `json:"error"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (request %s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code ErrorCode, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e *Error) WithDetail(key, value string) *Error {
	if e.Details == nil {
		e.Details = map[string]string{}
	}
	e.Details[key] = value
	return e
}

func (e *Error) WithRequestID(id string) *Error {
	e.RequestID = id
	return e
}

func (e *Error) WithRetryable(retryable bool) *Error {
	e.Retryable = retryable
	return e
}

func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Never expose unknown error text to clients.
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := AsError(err); ok {
		return e
	}
	return &Error{Code: CodeInternal, Message: "internal error"}
}

func StatusForCode(code ErrorCode) int {
	switch code {
	case CodeBadRequest, CodeClientConfig, CodeHeadscaleNotConfigured:
		return http.StatusBadRequest
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied, CodePeerNotAuthorized:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case CodeConflict, CodeOperationInProgress, CodeServiceAlreadyStarted:
		return http.StatusConflict
	case CodeProtocolIncompatible:
		return http.StatusUpgradeRequired
	case CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeRequestTimeout:
		return http.StatusGatewayTimeout
	case CodeUnsupported, CodeServiceCtlUnavailable, CodeHeadscaleVersionBad:
		return http.StatusNotImplemented
	case CodeHeadscaleUnavailable, CodeDaemonUnavailable, CodeDaemonNotRunning,
		CodeServiceNotInstalled, CodeDependencyUnavailable:
		return http.StatusServiceUnavailable
	case CodeHeadscaleUnauthorized:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func RetryableForCode(code ErrorCode) bool {
	switch code {
	case CodeHeadscaleUnavailable, CodeDaemonUnavailable, CodeRateLimited,
		CodeRequestTimeout, CodeDependencyUnavailable, CodeOperationInProgress:
		return true
	default:
		return false
	}
}
