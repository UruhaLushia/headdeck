package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
)

type apiCall func(*http.Request) (any, error)

func (s *Server) endpoint(mux *http.ServeMux, route string, permissions []principal.Permission, call apiCall, afterResponse ...func(any)) {
	mux.HandleFunc(route, s.withSession(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r)
		if !ok {
			s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "no session"))
			return
		}
		for _, permission := range permissions {
			if !p.Has(permission) {
				s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "this session may not perform that operation"))
				return
			}
		}
		out, err := call(r)
		if r.Method != http.MethodGet {
			result := "ok"
			if err != nil {
				result = "error"
			}
			s.logger.Audit(r.Context(), log.AuditEvent{Actor: p.String(), Action: route, Resource: r.PathValue("id"), Result: result})
		}
		if err != nil {
			s.writeError(w, protocol.FromError(err))
			return
		}
		status := http.StatusOK
		if _, ok := out.(*protocol.Operation); ok {
			status = http.StatusAccepted
		}
		s.writeJSON(w, status, out)
		if len(afterResponse) > 0 {
			if err := http.NewResponseController(w).Flush(); err != nil {
				s.logger.Named("web").Debug("response flush failed", "error", err.Error())
			}
			for _, after := range afterResponse {
				after(out)
			}
		}
	}))
}

func decodeRequest[T any](r *http.Request) (T, error) {
	var value T
	body := io.LimitReader(r.Body, (1<<20)+1)
	data, err := io.ReadAll(body)
	if err != nil || len(data) > 1<<20 {
		return value, protocol.NewError(protocol.CodeBadRequest, "request body exceeds the limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, protocol.NewError(protocol.CodeBadRequest, "invalid request body")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return value, protocol.NewError(protocol.CodeBadRequest, "request body must contain one JSON value")
	}
	return value, nil
}

func withBody[T any](call func(*http.Request, T) (any, error)) apiCall {
	return func(r *http.Request) (any, error) {
		value, err := decodeRequest[T](r)
		if err != nil {
			return nil, err
		}
		return call(r, value)
	}
}

func actor(r *http.Request) string {
	p, ok := principalFrom(r)
	if !ok {
		return "anonymous"
	}
	return p.String()
}
