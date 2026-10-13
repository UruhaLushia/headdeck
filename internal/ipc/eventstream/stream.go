package eventstream

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/events"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

const (
	heartbeat    = 20 * time.Second
	writeTimeout = 10 * time.Second
	maxLifetime  = 30 * time.Minute
)

func Scopes(p *principal.Principal) []string {
	if p == nil {
		return nil
	}
	var scopes []string
	for _, scope := range []struct {
		permission principal.Permission
		event      string
	}{
		{principal.PermNodesRead, protocol.EventNodesChanged},
		{principal.PermUsersRead, protocol.EventUsersChanged},
		{principal.PermKeysRead, protocol.EventKeysChanged},
		{principal.PermRoutesRead, protocol.EventRoutesChanged},
		{principal.PermOperationsRead, protocol.EventOperationChanged},
		{principal.PermSystemRead, protocol.EventHeadscaleRecovered},
		{principal.PermDNSRead, protocol.EventDNSChanged},
		{principal.PermPolicyRead, protocol.EventPolicyChanged},
		{principal.PermSettingsRead, protocol.EventConfigChanged},
		{principal.PermAccountsRead, protocol.EventAccountsChanged},
		{principal.PermAccountsRead, protocol.EventProvidersChanged},
	} {
		if p.Has(scope.permission) {
			scopes = append(scopes, scope.event)
		}
	}
	return scopes
}

func Serve(w http.ResponseWriter, r *http.Request, hub *events.Hub, scopes []string, lifetime time.Duration, authorized func() bool) *protocol.Error {
	lastID, err := strconv.ParseUint(strings.TrimSpace(r.Header.Get("Last-Event-ID")), 10, 64)
	if r.Header.Get("Last-Event-ID") == "" {
		lastID, err = 0, nil
	}
	if err != nil {
		return protocol.NewError(protocol.CodeBadRequest, "Last-Event-ID must be a non-negative integer")
	}
	release, admitted := hub.AcquireStream()
	if !admitted {
		return protocol.NewError(protocol.CodeRateLimited, "too many event streams").WithRetryable(true)
	}
	defer release()
	writer, perr := NewWriter(w, r, lifetime, authorized)
	if perr != nil {
		return perr
	}
	replay, ch, cancel, cursor, ok := hub.SubscribeFrom(lastID)
	defer cancel()
	write := writer.Write
	resync := func(reason string) { _ = write("resync", hub.LatestID(), map[string]string{"reason": reason}) }
	if !ok {
		resync("history_expired")
		return nil
	}
	if err := write("hello", 0, map[string]any{"scopes": scopes, "instance": hub.Instance()}); err != nil {
		return nil
	}
	allowed := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		allowed[scope] = true
	}
	send := func(ev events.Event) error {
		if !allowed[ev.Type] {
			return nil
		}
		return write(ev.Type, ev.ID, protocol.Event{
			ID: ev.ID, Type: ev.Type, Timestamp: ev.Timestamp.UTC().Format(time.RFC3339),
			ResourceID: ev.ResourceID, Revision: ev.Revision,
			Data: ev.Data,
		})
	}
	for i, ev := range replay {
		replay[i] = events.Event{}
		if err := send(ev); err != nil {
			return nil
		}
	}
	if err := write("cursor", cursor, struct{}{}); err != nil {
		return nil
	}
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	timer := time.NewTimer(time.Until(writer.ExpiresAt))
	defer timer.Stop()
	recheck := time.NewTicker(time.Second)
	defer recheck.Stop()
	for {
		select {
		case <-recheck.C:
			if !authorized() {
				return nil
			}
		case <-r.Context().Done():
			return nil
		case <-timer.C:
			_ = write("bye", 0, map[string]string{"reason": "lifetime"})
			return nil
		case ev, open := <-ch:
			if !open {
				resync("subscriber_overflow")
				return nil
			}
			cursor = ev.ID
			if err := send(ev); err != nil {
				return nil
			}
		case <-ticker.C:
			if err := write("cursor", cursor, struct{}{}); err != nil {
				return nil
			}
		}
	}
}
