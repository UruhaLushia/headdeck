package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func withPrincipal(r *http.Request, p *principal.Principal) context.Context {
	return principal.WithContext(r.Context(), p)
}

func principalFrom(r *http.Request) (*principal.Principal, bool) {
	return principal.FromContext(r.Context())
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && acceptsJSON(r) {
		s.writeAPIIndex(w)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.writeError(w, protocol.NewError(protocol.CodeNotFound, "unknown endpoint"))
		return
	}
	if s.serveStatic(w, r) {
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Accept") != "" && !acceptsJSON(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, placeholderPage)
		return
	}
	s.writeAPIIndex(w)
}

func (s *Server) writeAPIIndex(w http.ResponseWriter) {
	note := "run `pnpm --dir web build` and rebuild hdkt to serve the browser UI"
	if _, ok := staticAssets(); ok {
		note = "the browser UI is served at /; this index lists the JSON API"
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"service": "headdeck-web",
		"note":    note,
		"auth":    "POST /api/session with {\"token\":\"<server.web.token_file contents>\"}",
		"endpoints": []string{
			"GET /api/session", "DELETE /api/session", "GET /api/status", "GET /api/status/stream (SSE)",
			"GET /api/nodes", "GET /api/users", "GET /api/routes",
			"GET /api/preauth-keys", "GET /api/events",
		},
	})
}

func acceptsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	for i := 0; i+4 <= len(accept); i++ {
		if accept[i:i+4] == "json" {
			return true
		}
	}
	return false
}

const placeholderPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Headdeck</title>
<style>
body { font: 15px/1.5 system-ui, sans-serif; margin: 3rem auto; max-width: 44rem; padding: 0 1rem; }
code { background: #f2f2f2; padding: .1rem .3rem; border-radius: 3px; }
</style>
</head>
<body>
<h1>Headdeck</h1>
<p>The daemon is running and the browser API is available, but this binary was
built without the browser UI. Build it with <code>pnpm --dir web build</code> and
rebuild <code>hdkt</code>. The terminal UI (<code>hdkt</code>) and the CLI work
either way.</p>
<h2>API</h2>
<p>Sign in first: <code>POST /api/session</code> with
<code>{"token":"&lt;contents of server.web.token_file&gt;"}</code>. The response sets an
HttpOnly session cookie. Every other endpoint requires it, and non-GET requests
also need the <code>X-Requested-With: headdeck</code> header.</p>
<ul>
<li><code>GET /api/status</code></li>
<li><code>GET /api/nodes</code></li>
<li><code>GET /api/users</code></li>
<li><code>GET /api/routes</code></li>
<li><code>GET /api/preauth-keys</code></li>
<li><code>GET /api/events</code> (server-sent events)</li>
</ul>
</body>
</html>
`
