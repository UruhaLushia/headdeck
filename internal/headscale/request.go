package headscale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out, true)
}

// Do not retry writes: a lost response leaves their outcome unknown.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any, idempotent bool) error {
	if c.baseURL == "" {
		return ErrNotConfigured
	}
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	attempts := 1
	if idempotent && c.maxRetries > 0 {
		attempts = c.maxRetries + 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		resp, err := c.attempt(ctx, method, path, query, payload)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.retryable {
			lastErr = resp.err
			continue
		}
		if resp.err != nil {
			return resp.err
		}
		if out != nil && resp.raw != nil {
			if err := json.Unmarshal(resp.raw, out); err != nil {
				return fmt.Errorf("decode headscale response for %s: %w", path, err)
			}
		}
		return nil
	}
	return lastErr
}

var ErrNotConfigured = errors.New("headscale base url is not configured")

type attemptResult struct {
	raw       []byte
	err       error
	retryable bool
}

func (c *Client) attempt(ctx context.Context, method, path string, query url.Values, payload []byte) (attemptResult, error) {
	key := ""
	if c.rpc == nil {
		var err error
		key, err = c.APIKey()
		if err != nil {
			return attemptResult{}, err
		}
	} else if method != http.MethodGet || path != "/version" {
		return attemptResult{}, fmt.Errorf("headscale management requires the Unix gRPC interface")
	}
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return attemptResult{}, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return attemptResult{}, &TransportError{Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return attemptResult{}, &TransportError{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parseAPIError(resp.StatusCode, raw)
		return attemptResult{
			err:       apiErr,
			retryable: apiErr.Retryable(),
		}, nil
	}
	return attemptResult{raw: raw}, nil
}

func parseAPIError(status int, raw []byte) *APIError {
	err := &APIError{StatusCode: status}
	var legacy struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &legacy) == nil && (legacy.Message != "" || legacy.Code != 0) {
		err.GRPCCode = legacy.Code
		err.Message = legacy.Message
		return err
	}
	var problem struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Status int    `json:"status"`
	}
	if json.Unmarshal(raw, &problem) == nil && (problem.Title != "" || problem.Detail != "") {
		err.RFC7807 = true
		err.Message = strings.TrimSpace(problem.Title + " " + problem.Detail)
		return err
	}
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	err.Message = msg
	return err
}
