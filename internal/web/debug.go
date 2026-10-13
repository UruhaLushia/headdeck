package web

import (
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) withDiagnostics(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := s.settings.Load(r.Context())
		if err != nil {
			s.writeError(w, protocol.FromError(err))
			return
		}
		if !cfg.Server.Diagnostics {
			s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "diagnostics are disabled"))
			return
		}
		next(w, r)
	}
}

func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
	if raw := r.URL.Query().Get("seconds"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 1 || seconds > 30 {
			s.writeError(w, protocol.NewError(protocol.CodeBadRequest, "profile duration must be an integer from 1 to 30 seconds"))
			return
		}
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/debug/pprof/")
	switch name {
	case "":
		pprof.Index(w, r)
	case "cmdline":
		pprof.Cmdline(w, r)
	case "profile":
		pprof.Profile(w, r)
	case "symbol":
		pprof.Symbol(w, r)
	case "trace":
		pprof.Trace(w, r)
	default:
		pprof.Handler(name).ServeHTTP(w, r)
	}
}
