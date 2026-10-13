package github

import (
	"errors"
	"io"
	"net/http"
)

type transport struct{ base http.RoundTripper }

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.User != nil || (req.URL.Host != "github.com" && req.URL.Host != "api.github.com") {
		return nil, errors.New("invalid GitHub endpoint")
	}
	req = req.Clone(req.Context())
	if req.URL.Host == "github.com" {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "Headdeck")
	response, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if response.ContentLength > 1<<20 {
		response.Body.Close()
		return nil, errors.New("GitHub response exceeds size limit")
	}
	response.Body = &limitedBody{ReadCloser: response.Body, remaining: 1 << 20}
	return response, nil
}

type limitedBody struct {
	io.ReadCloser
	remaining int
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, errors.New("GitHub response exceeds size limit")
	}
	if len(p) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= n
	return n, err
}
