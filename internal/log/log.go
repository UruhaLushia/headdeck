package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	subsystemKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func WithSubsystem(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, subsystemKey, name)
}

func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func Subsystem(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	name, _ := ctx.Value(subsystemKey).(string)
	return name
}

type Logger struct {
	base  *slog.Logger
	audit *slog.Logger
}

type Options struct {
	Level       string
	JSON        bool
	Writer      io.Writer
	AuditWriter io.Writer
	AuditJSON   bool
}

func New(opts Options) *Logger {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	level := ParseLevel(opts.Level)
	handlerOpts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(w, handlerOpts)
	} else {
		h = slog.NewTextHandler(w, handlerOpts)
	}
	aw := opts.AuditWriter
	if aw == nil {
		aw = w
	}
	auditHandler := slog.NewJSONHandler(aw, &slog.HandlerOptions{Level: slog.LevelInfo})
	return &Logger{base: slog.New(h), audit: slog.New(auditHandler)}
}

func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug", "trace":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l *Logger) Named(name string) *slog.Logger {
	return l.base.With(slog.String("subsystem", name))
}

func (l *Logger) Slog() *slog.Logger { return l.base }

type AuditEvent struct {
	OperationID string
	Reason      string
	Actor       string
	Action      string
	Resource    string
	Result      string
	RequestID   string
	PeerUID     *uint32
	Error       string
}

func (l *Logger) Audit(ctx context.Context, ev AuditEvent) {
	attrs := []any{
		slog.String("audit", "true"),
		slog.String("actor", ev.Actor),
		slog.String("action", ev.Action),
		slog.String("result", ev.Result),
	}
	if ev.OperationID != "" {
		attrs = append(attrs, slog.String("operation_id", ev.OperationID))
	}
	if ev.Reason != "" {
		attrs = append(attrs, slog.String("reason", Redact(ev.Reason)))
	}
	if ev.Resource != "" {
		attrs = append(attrs, slog.String("resource", ev.Resource))
	}
	if id := ev.RequestID; id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	} else if id := RequestID(ctx); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	if ev.PeerUID != nil {
		attrs = append(attrs, slog.Uint64("peer_uid", uint64(*ev.PeerUID)))
	}
	if ev.Error != "" {
		attrs = append(attrs, slog.String("error", Redact(ev.Error)))
	}
	l.audit.Log(ctx, slog.LevelInfo, "audit", attrs...)
}
