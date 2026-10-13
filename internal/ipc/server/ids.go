package server

import (
	"crypto/rand"
	"encoding/hex"
)

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}
