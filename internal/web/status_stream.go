package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/eventstream"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) handleStatusStream(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && !s.originAllowed(origin) {
		s.writeError(w, protocol.NewError(protocol.CodePermissionDenied, "invalid event stream origin"))
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return
	}
	p, session, valid := s.sessionPrincipal(r, cookie.Value)
	if !valid {
		return
	}
	release, admitted := s.svc.Hub().AcquireStream()
	if !admitted {
		s.writeError(w, protocol.NewError(protocol.CodeRateLimited, "too many event streams"))
		return
	}
	defer release()
	authorized := func() bool {
		if err := s.authorizeLogin(r.Context()); err != nil {
			return false
		}
		current, _, ok := s.sessionPrincipal(r, cookie.Value)
		return ok && current.ID == p.ID && current.Role == p.Role && current.HeadscaleUserID == p.HeadscaleUserID && current.Has(principal.PermSystemRead)
	}
	writer, perr := eventstream.NewWriter(w, r, session.ExpiresAt.Sub(s.now()), authorized)
	if perr != nil {
		s.writeError(w, perr)
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), writer.ExpiresAt)
	defer cancel()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	recheck := time.NewTicker(time.Second)
	defer recheck.Stop()
	var previous map[string]json.RawMessage
	if err := s.sendStatus(ctx, writer, p, &previous); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-recheck.C:
			if !authorized() {
				return
			}
		case <-ticker.C:
			if err := s.sendStatus(ctx, writer, p, &previous); err != nil {
				return
			}
		}
	}
}

func (s *Server) sendStatus(ctx context.Context, writer *eventstream.Writer, p *principal.Principal, previous *map[string]json.RawMessage) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	status, err := s.status(ctx, p, false)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	var next map[string]json.RawMessage
	if err := json.Unmarshal(raw, &next); err != nil {
		return err
	}
	changed := make(map[string]json.RawMessage)
	for key, value := range next {
		if !bytes.Equal(value, (*previous)[key]) {
			changed[key] = value
		}
	}
	// A removed field must overwrite its previous value on the client.
	for key := range *previous {
		if _, exists := next[key]; !exists {
			changed[key] = json.RawMessage("null")
		}
	}
	kind := "diff"
	if *previous == nil {
		kind = "snapshot"
	}
	*previous = next
	if len(changed) == 0 {
		return nil
	}
	return writer.Write(kind, 0, changed)
}
