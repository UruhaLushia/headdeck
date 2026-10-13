//go:build !windows

package transport

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"
)

func listenLocal(ctx context.Context, path string) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(ctx, "unix", path)
}

func syscallConnRefused() error { return syscall.ECONNREFUSED }

func checkOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (stat.Uid != 0 && stat.Uid != uint32(os.Geteuid())) {
		return fmt.Errorf("IPC path must be owned by root or the daemon UID")
	}
	return nil
}
