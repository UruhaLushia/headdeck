package client

import (
	"time"
)

func WatchBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	delay := time.Duration(1<<uint(min(attempt, 5))) * 250 * time.Millisecond
	if delay > 10*time.Second {
		delay = 10 * time.Second
	}
	return delay
}
