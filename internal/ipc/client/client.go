package client

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/ipc/transport"
)

const maxResponseBytes = 8 << 20

const localHost = "http://" + protocol.LocalHost

type Options struct {
	SocketPath string
	Timeout    time.Duration
	ClientName string
	UserAgent  string
}

type StatusError struct {
	StatusCode int
	Proto      *protocol.Error
}

func (e *StatusError) Error() string {
	if e.Proto != nil {
		return e.Proto.Error()
	}
	return fmt.Sprintf("daemon returned HTTP %d", e.StatusCode)
}

func (e *StatusError) Code() protocol.ErrorCode {
	if e.Proto == nil {
		return ""
	}
	return e.Proto.Code
}

type Client struct {
	handshakeMu   sync.Mutex
	ready         atomic.Bool
	eventInstance atomic.Pointer[string]
	socket        string
	http          *http.Client
	clientName    string
	userAgent     string
}

func New(opts Options) (*Client, error) {
	if opts.SocketPath == "" {
		return nil, protocol.NewError(protocol.CodeClientConfig, "socket path must not be empty")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	name := opts.ClientName
	if name == "" {
		name = "hdkt"
	}
	tr := transport.NewHTTPTransport(opts.SocketPath, timeout)
	return &Client{
		socket:     opts.SocketPath,
		http:       &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		clientName: name,
		userAgent:  opts.UserAgent,
	}, nil
}

func (c *Client) SocketPath() string { return c.socket }

func (c *Client) HTTPClient() *http.Client { return c.http }
