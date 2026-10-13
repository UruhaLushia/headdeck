package web

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func headdeckRoute(path string) bool {
	if path == "/oidc/callback" || path == "/api/v1" || strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	return path == "/" || path == "/index.html" || path == "/favicon.ico" ||
		strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/api/") ||
		path == "/oidc" || strings.HasPrefix(path, "/oidc/")
}

func (s *Server) serveHeadscale(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/oidc/callback" || strings.HasPrefix(r.URL.Path, "/register/") || strings.HasPrefix(r.URL.Path, "/auth/") {
		if err := s.authorizeLogin(r.Context()); err != nil {
			http.Error(w, "Headscale login takeover is not ready", http.StatusServiceUnavailable)
			return
		}
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		http.Error(w, "Cannot start Headscale stream", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Upgrade") != "" {
		if err := controller.SetReadDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "Cannot start Headscale stream", http.StatusInternalServerError)
			return
		}
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(s.proxyContext, cancel)
	defer stop()
	defer cancel()
	s.headscaleProxy.ServeHTTP(w, r.WithContext(ctx))
}

func (s *Server) initHeadscaleProxy() error {
	u, err := url.Parse(s.svc.Config().Headscale.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.Scheme != "http" && u.Scheme != "https" || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("headscale proxy requires an HTTP(S) upstream without a path")
	}
	public, err := url.Parse(s.cfg.PublicURL)
	if err != nil || public.Host == "" || public.Path != "" && public.Path != "/" || strings.EqualFold(public.Host, u.Host) {
		return errors.New("headscale proxy requires a public domain root separate from its upstream")
	}
	transport := &http.Transport{
		DialContext:  (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns: 100, MaxIdleConnsPerHost: 32, IdleConnTimeout: 90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: s.svc.Config().Headscale.InsecureSkipVerify},
	}
	s.proxyTransport = transport
	s.proxyContext, s.stopProxy = context.WithCancel(context.Background())
	s.headscaleProxy = &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(u)
			p.Out.Host = public.Host
			clientIP := s.clientIP(p.In)
			p.Out.Header.Set("X-Forwarded-For", clientIP)
			p.Out.Header.Set("X-Real-IP", clientIP)
			p.Out.Header.Set("X-Forwarded-Host", public.Host)
			p.Out.Header.Set("X-Forwarded-Proto", public.Scheme)
			p.Out.Header.Del("Cookie")
			for _, cookie := range p.In.Cookies() {
				if !strings.HasPrefix(cookie.Name, "headdeck_") {
					p.Out.AddCookie(cookie)
				}
			}
		},
		ModifyResponse: func(r *http.Response) error {
			cookies := r.Cookies()
			r.Header.Del("Set-Cookie")
			for _, cookie := range cookies {
				if !strings.HasPrefix(cookie.Name, "headdeck_") {
					r.Header.Add("Set-Cookie", cookie.String())
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.logger.Named("web").Debug("Headscale proxy failed", "error", err.Error())
			http.Error(w, "Headscale is unavailable", http.StatusBadGateway)
		},
	}
	return nil
}
