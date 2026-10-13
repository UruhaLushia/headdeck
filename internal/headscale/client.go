package headscale

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const maxResponseBytes = 4 << 20

type Options struct {
	BaseURL            string
	APIKey             string
	Timeout            time.Duration
	MaxRetries         int
	InsecureSkipVerify bool
	UserAgent          string
	UnixSocket         string
}

type Client struct {
	baseURL    string
	timeout    time.Duration
	maxRetries int
	http       *http.Client
	userAgent  string

	key string
	rpc *grpc.ClientConn
}

func New(opts Options) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if base != "" {
		if _, err := url.Parse(base); err != nil {
			return nil, fmt.Errorf("headscale base url %q is invalid: %w", opts.BaseURL, err)
		}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	tr := &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify},
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "headdeck/0.1"
	}
	c := &Client{
		baseURL:    base,
		timeout:    timeout,
		maxRetries: opts.MaxRetries,
		http: &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		userAgent: ua,
		key:       strings.TrimSpace(opts.APIKey),
	}
	if opts.UnixSocket != "" {
		if !filepath.IsAbs(opts.UnixSocket) || filepath.Clean(opts.UnixSocket) != opts.UnixSocket {
			return nil, fmt.Errorf("headscale Unix socket must use a clean absolute path")
		}
		conn, err := grpc.NewClient("unix://"+opts.UnixSocket,
			grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry(),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxResponseBytes)))
		if err != nil {
			return nil, err
		}
		c.rpc = conn
		c.key = ""
	}
	return c, nil
}

func (c *Client) Close() {
	if c.rpc != nil {
		c.rpc.Close()
	}
	c.http.CloseIdleConnections()
}

func (c *Client) BaseURL() string { return c.baseURL }
