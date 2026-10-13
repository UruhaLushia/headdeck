package headscale

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// ReadDiagnostic uses a separate local listener and never sends management credentials.
func (c *Client) ReadDiagnostic(ctx context.Context, base, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain;q=0.9")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Headscale diagnostics")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("headscale diagnostic endpoint returned HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read Headscale diagnostic response")
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("headscale diagnostic response exceeds 4 MiB")
	}
	return data, nil
}
