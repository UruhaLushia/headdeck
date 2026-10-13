package web

import (
	"embed"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var dist embed.FS

func staticAssets() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return sub, true
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) bool {
	assets, ok := staticAssets()
	if !ok {
		return false
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}

	if _, err := fs.Stat(assets, name); err != nil {

		s.serveIndex(w, r, assets)
		return true
	}

	if name == "index.html" {
		s.serveIndex(w, r, assets)
		return true
	}
	if strings.HasPrefix(name, "assets/") {

		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, assets, name)
	return true
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	body, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	markup := strings.Replace(string(body), "<head>", `<head><base href="`+html.EscapeString(s.basePath()+"/")+`">`, 1)
	http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(markup))
}
