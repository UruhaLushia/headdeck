package web

import (
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) basePath() string {
	u, err := url.Parse(s.cfg.PublicURL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.Path, "/")
}

func (s *Server) routeRequest(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	prefix := s.basePath()
	if prefix == "" {
		return r, true
	}
	if r.URL.Path == prefix {
		location := prefix + "/"
		if r.URL.RawQuery != "" {
			location += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, location, http.StatusPermanentRedirect)
		return nil, false
	}
	if !strings.HasPrefix(r.URL.Path, prefix+"/") {
		http.NotFound(w, r)
		return nil, false
	}
	next := r.Clone(r.Context())
	next.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
	next.URL.RawPath = ""
	return next, true
}
