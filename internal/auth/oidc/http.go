package oidc

import (
	"errors"
	"io"
	"net"
	"net/http"

	"net/url"
)

type limitedTransport struct{ base http.RoundTripper }

func (t limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := safeURL(req.URL); err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	const limit = 1 << 20
	if response.ContentLength > limit {
		response.Body.Close()
		return nil, errors.New("OIDC response exceeds size limit")
	}
	response.Body = &limitedBody{ReadCloser: response.Body, remaining: limit}
	return response, nil
}

func safeURL(u *url.URL) error {
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	if u.User != nil || u.Host == "" || u.Scheme != "https" && (u.Scheme != "http" || !local) {
		return errors.New("OIDC endpoints require HTTPS except on loopback")
	}
	return nil
}

type limitedBody struct {
	io.ReadCloser
	remaining int
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, errors.New("OIDC response exceeds size limit")
	}
	if len(p) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= n
	return n, err
}
