package web

import (
	"net"
	"net/http"
	"strings"
)

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// X-Forwarded-For is only believed from an explicitly trusted proxy.
	if s.trustedProxy(host) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			hops := strings.Split(forwarded, ",")
			for i := len(hops) - 1; i >= 0; i-- {
				candidate := strings.TrimSpace(hops[i])
				if net.ParseIP(candidate) == nil {
					return host
				}
				if !s.trustedProxy(candidate) {
					return candidate
				}
			}
		}
	}
	return host
}

func (s *Server) trustedProxy(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range s.cfg.TrustedProxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
