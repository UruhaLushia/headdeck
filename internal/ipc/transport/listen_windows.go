//go:build windows

package transport

import (
	"context"
	"net"
	"os"
)

func listenLocal(context.Context, string) (net.Listener, error) {
	return nil, ErrUnsupportedPlatform
}

func syscallConnRefused() error { return nil }

func checkOwner(os.FileInfo) error { return ErrUnsupportedPlatform }
