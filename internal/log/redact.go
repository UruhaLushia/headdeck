package log

import (
	"strings"
	"sync"
)

func Redact(s string) string {
	if s == "" {
		return s
	}
	redactor.mu.Lock()
	defer redactor.mu.Unlock()
	for _, m := range redactor.markers {
		s = maskAfter(s, m)
	}
	return s
}

type redactorState struct {
	mu      sync.Mutex
	markers []string
}

var redactor = redactorState{
	markers: []string{"Authorization: Bearer ", "HEADDECK-AUTH-V2", "hskey-", "Bearer "},
}

func AddMarker(prefix string) {
	redactor.mu.Lock()
	defer redactor.mu.Unlock()
	redactor.markers = append(redactor.markers, prefix)
}

func maskAfter(s, marker string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, marker)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(marker)
		b.WriteString(s[:end])
		b.WriteString("***")
		j := end
		for j < len(s) && !isTokenBoundary(s[j]) {
			j++
		}
		s = s[j:]
	}
}

func isTokenBoundary(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '"', '\'', ',', ';', ')', ']', '}':
		return true
	default:
		return false
	}
}
