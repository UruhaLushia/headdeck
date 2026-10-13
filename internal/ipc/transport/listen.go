package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrInUse               = errors.New("socket path is already in use by a running daemon")
	ErrUnsupportedPlatform = errors.New("the local IPC transport is only implemented for unix systems")
)

type ListenOptions struct {
	Path string
	// The parent directory must protect the bind-to-chmod window.
	Mode        os.FileMode
	DirMode     os.FileMode
	RemoveStale bool
}

// Reject regular files, symlinks and live sockets.
func Listen(ctx context.Context, opts ListenOptions) (net.Listener, error) {
	path, err := validateSocketPath(opts.Path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if opts.DirMode == 0 {
		opts.DirMode = 0o750
	}
	if info, err := os.Lstat(dir); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(dir, opts.DirMode); err != nil {
			return nil, fmt.Errorf("create runtime directory %s: %w", dir, err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("runtime path %s exists and is not a directory", dir)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("runtime directory %s must not be writable by group or others", dir)
	}
	if err := checkOwner(info); err != nil {
		return nil, fmt.Errorf("runtime directory %s: %w", dir, err)
	}

	if err := preparePath(ctx, path, opts.RemoveStale); err != nil {
		return nil, err
	}

	ln, err := listenLocal(ctx, path)
	if err != nil {
		return nil, err
	}
	if opts.Mode != 0 {
		if err := os.Chmod(path, opts.Mode); err != nil {
			ln.Close() //nolint:gosec // the listener is being abandoned
			return nil, fmt.Errorf("set socket mode %04o: %w", opts.Mode, err)
		}
	}
	return ln, nil
}

func validateSocketPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("socket path must not be empty")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("socket path %q must be absolute", path)
	}
	if len(clean) > 100 {
		return "", fmt.Errorf("socket path %q is too long for a Unix socket", clean)
	}
	return clean, nil
}

func preparePath(ctx context.Context, path string, removeStale bool) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to use %s: it is a symlink", path)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace %s: it exists and is not a socket", path)
	}
	if err := checkOwner(info); err != nil {
		return fmt.Errorf("existing socket %s: %w", path, err)
	}
	live, err := socketIsLive(ctx, path)
	if err == nil && live {
		return fmt.Errorf("%w: %s", ErrInUse, path)
	}
	if err != nil && !errors.Is(err, syscallConnRefused()) && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("probe existing socket %s: %w", path, err)
	}
	if !removeStale {
		return fmt.Errorf("stale socket %s exists (stale socket removal is disabled)", path)
	}
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return fmt.Errorf("socket %s changed while probing", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale socket %s: %w", path, err)
	}
	return nil
}

func socketIsLive(ctx context.Context, path string) (bool, error) {
	dialer := &net.Dialer{Timeout: 300 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return false, err
	}
	conn.Close() //nolint:gosec // probe connection, closed immediately
	return true, nil
}
