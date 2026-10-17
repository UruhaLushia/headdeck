package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/charmbracelet/x/ansi"
)

const maxLogBytes = 2 << 20

type journalOutput struct{ buffer bytes.Buffer }

func (b *journalOutput) Write(p []byte) (int, error) {
	if len(p) > maxLogBytes-b.buffer.Len() {
		return 0, errors.New("journal output exceeds limit")
	}
	return b.buffer.Write(p)
}

func (s *Service) Logs(ctx context.Context, source string) (*protocol.LogSnapshot, error) {
	var unit string
	switch source {
	case "headdeck":
		unit = paths.ServiceName
	case "headscale":
		unit = s.cfg.Features.ServiceUnit
	default:
		return nil, protocol.NewError(protocol.CodeBadRequest, "unknown log source")
	}
	if !config.ValidServiceUnit(unit) {
		return nil, protocol.NewError(protocol.CodeClientConfig, "invalid service unit")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// The source selects a local unit; callers cannot supply journal options or paths.
	cmd := exec.CommandContext(ctx, "journalctl", "--unit="+unit, "--lines=200", "--reverse",
		"--output=json", "--output-fields=__REALTIME_TIMESTAMP,PRIORITY,MESSAGE", "--all", "--no-pager", "--quiet")
	var output journalOutput
	cmd.Stdout = &output
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeRequestTimeout, "log request timed out")
		}
		return nil, protocol.NewError(protocol.CodeDependencyUnavailable, "cannot read service logs; check journal access")
	}
	out := &protocol.LogSnapshot{Source: source, CollectedAt: time.Now().UTC(), Entries: []protocol.LogEntry{}}
	decoder := json.NewDecoder(&output.buffer)
	for {
		var entry struct {
			Timestamp string          `json:"__REALTIME_TIMESTAMP"`
			Priority  string          `json:"PRIORITY"`
			Message   json.RawMessage `json:"MESSAGE"`
		}
		if err := decoder.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, protocol.NewError(protocol.CodeInternal, "invalid journal response")
		}
		var message string
		if err := json.Unmarshal(entry.Message, &message); err != nil {
			// journalctl encodes binary messages as byte arrays.
			var binary []byte
			if err := json.Unmarshal(entry.Message, &binary); err != nil {
				continue
			}
			message = string(binary)
		}
		micros, err := strconv.ParseInt(entry.Timestamp, 10, 64)
		if err != nil {
			continue
		}
		message = ansi.Strip(message)
		level := journalLevel(entry.Priority, message)
		out.Entries = append(out.Entries, protocol.LogEntry{
			Time: time.UnixMicro(micros).UTC(), Level: level, Message: log.Redact(message),
		})
	}
	return out, nil
}

func journalLevel(priority, message string) string {
	var structured struct{ Level string }
	if json.Unmarshal([]byte(message), &structured) == nil {
		switch strings.ToLower(structured.Level) {
		case "debug", "trace":
			return "debug"
		case "info":
			return "info"
		case "warn", "warning":
			return "warn"
		case "error", "fatal", "panic":
			return "error"
		}
	}
	for _, field := range strings.Fields(message) {
		switch field {
		case "level=DEBUG":
			return "debug"
		case "level=INFO":
			return "info"
		case "level=WARN":
			return "warn"
		case "level=ERROR":
			return "error"
		}
	}
	// Headscale's text formatter puts a three-letter level after the timestamp.
	if fields := strings.Fields(message); len(fields) > 1 {
		switch fields[1] {
		case "TRC", "DBG":
			return "debug"
		case "INF":
			return "info"
		case "WRN":
			return "warn"
		case "ERR", "FTL", "PNC":
			return "error"
		}
	}
	switch priority {
	case "0", "1", "2", "3":
		return "error"
	case "4":
		return "warn"
	case "7":
		return "debug"
	default:
		return "info"
	}
}
