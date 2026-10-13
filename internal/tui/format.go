package tui

import (
	"strings"
	"time"
)

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func onOffText(b bool) string {
	if b {
		return "online"
	}
	return "offline"
}

func shortTime(raw string) string {
	if raw == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Local().Format("01-02 15:04")
}
