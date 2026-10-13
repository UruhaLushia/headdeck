package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (c *Client) readEvents(body io.Reader, opts WatchOptions) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	var (
		eventType string
		data      strings.Builder
		cursor    uint64
		hasCursor bool
	)

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			payload := strings.TrimSpace(data.String())
			data.Reset()
			err := c.dispatchEvent(eventType, payload, opts)
			eventType = ""
			if hasCursor && (err == nil || errors.Is(err, io.EOF)) && opts.OnCursor != nil {
				opts.OnCursor(cursor)
			}
			hasCursor = false
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len()+len(line) > 1<<20 {
				return protocol.NewError(protocol.CodePayloadTooLarge, "event frame exceeds 1 MiB")
			}
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case strings.HasPrefix(line, "id:"):
			id, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "id:")), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid event cursor: %w", err)
			}
			cursor, hasCursor = id, true
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("event stream interrupted: %w", err)
	}
	return io.ErrUnexpectedEOF
}

func (c *Client) dispatchEvent(eventType, payload string, opts WatchOptions) error {
	if payload == "" {
		return nil
	}
	switch eventType {
	case "resync":
		if opts.OnResync != nil {
			opts.OnResync(resyncReason(payload))
		}
		return nil
	case "hello":
		var hello struct {
			Instance string `json:"instance"`
		}
		if err := json.Unmarshal([]byte(payload), &hello); err != nil || hello.Instance == "" {
			return fmt.Errorf("invalid event stream instance")
		}
		if previous := c.eventInstance.Swap(&hello.Instance); previous != nil && *previous != hello.Instance && opts.OnResync != nil {
			opts.OnResync("daemon_restarted")
		}
		return nil
	case "cursor":
		return nil
	case "bye":
		return io.EOF
	}
	var ev protocol.Event
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		return fmt.Errorf("invalid event payload: %w", err)
	}
	if ev.Type == "" {
		ev.Type = eventType
	}
	if opts.OnEvent != nil {
		return opts.OnEvent(ev)
	}
	return nil
}

func resyncReason(payload string) string {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(payload), &body); err != nil || body.Reason == "" {
		return "history_expired"
	}
	return body.Reason
}
