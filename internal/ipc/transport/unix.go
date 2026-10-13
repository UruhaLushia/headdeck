//go:build !windows

package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/sys/unix"
)

type PeerCred struct {
	PID int32
	UID uint32
	GID uint32
}

// Never substitute HTTP headers for kernel peer credentials.
func PeerCredFromConn(conn net.Conn) (*PeerCred, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("connection is %T, not a unix socket", conn)
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return nil, err
	}
	var cred *unix.Ucred
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		cred, sockErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return nil, err
	}
	if sockErr != nil {
		return nil, sockErr
	}
	if cred == nil {
		return nil, fmt.Errorf("kernel returned no peer credentials")
	}
	return &PeerCred{PID: cred.Pid, UID: cred.Uid, GID: cred.Gid}, nil
}

func NewHTTPTransport(path string, timeout time.Duration) *http.Transport {
	d := &net.Dialer{Timeout: timeout}
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "unix", path)
		},
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    true,
		ForceAttemptHTTP2:     false,
	}
}
