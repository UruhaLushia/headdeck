package eventstream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type Writer struct {
	w          http.ResponseWriter
	r          *http.Request
	ExpiresAt  time.Time
	authorized func() bool
}

func NewWriter(w http.ResponseWriter, r *http.Request, lifetime time.Duration, authorized func() bool) (*Writer, *protocol.Error) {
	if _, ok := w.(http.Flusher); !ok {
		return nil, protocol.NewError(protocol.CodeInternal, "this transport cannot stream events")
	}
	if lifetime <= 0 || lifetime > maxLifetime {
		lifetime = maxLifetime
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	return &Writer{w: w, r: r, ExpiresAt: time.Now().Add(lifetime), authorized: authorized}, nil
}

func (s *Writer) Write(kind string, id uint64, data any) error {
	if err := s.r.Context().Err(); err != nil {
		return err
	}
	if !time.Now().Before(s.ExpiresAt) || !s.authorized() {
		return fmt.Errorf("event stream no longer authorized")
	}
	deadline := time.Now().Add(writeTimeout)
	if s.ExpiresAt.Before(deadline) {
		deadline = s.ExpiresAt
	}
	controller := http.NewResponseController(s.w)
	if err := controller.SetWriteDeadline(deadline); err != nil && err != http.ErrNotSupported {
		return err
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("event: %s\ndata: %s\n", kind, body)
	if kind != "hello" && kind != "bye" && kind != "snapshot" && kind != "diff" {
		frame += fmt.Sprintf("id: %d\n", id)
	}
	if _, err := fmt.Fprint(s.w, frame+"\n"); err != nil {
		return err
	}
	return controller.Flush()
}
