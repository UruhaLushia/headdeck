package protocol

import "time"

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type LogSnapshot struct {
	Source      string     `json:"source"`
	CollectedAt time.Time  `json:"collected_at"`
	Entries     []LogEntry `json:"entries"`
}
