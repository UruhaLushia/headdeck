package client

import (
	"crypto/rand"
	"encoding/hex"
)

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return "cli-" + hex.EncodeToString(b[:])
}
