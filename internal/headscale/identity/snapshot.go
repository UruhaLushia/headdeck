package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/UruhaLushia/headdeck/internal/securefile"
)

const Directory = "/var/lib/headscale"
const databaseName = "db.sqlite"
const maxDatabaseSize = 512 << 20

func Apply(ctx context.Context, uid uint32, bindings []Binding, restore bool) (bool, error) {
	return applyIn(ctx, Directory, paths.StateDir+"/headscale-login", uid, bindings, restore)
}

func applyIn(ctx context.Context, directory, workspace string, uid uint32, bindings []Binding, restore bool) (bool, error) {
	if err := securefile.Directory(filepath.Dir(directory)); err != nil {
		return false, err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode().Perm()&0o022 != 0 || !ownedBy(info, uid) {
		return false, fmt.Errorf("unsafe Headscale database directory")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return false, err
	}
	defer root.Close()
	if err := checkSidecars(root); err != nil {
		return false, err
	}
	entry, err := root.Lstat(databaseName)
	if err != nil {
		return false, err
	}
	if !entry.Mode().IsRegular() {
		return false, fmt.Errorf("headscale database must be a regular file")
	}
	source, err := root.OpenFile(databaseName, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false, err
	}
	defer source.Close()
	info, err = source.Stat()
	if err != nil {
		return false, err
	}
	if !os.SameFile(entry, info) || !info.Mode().IsRegular() || info.Mode().Perm()&0o133 != 0 || !ownedBy(info, uid) || info.Size() > maxDatabaseSize {
		return false, fmt.Errorf("unsafe or oversized Headscale database")
	}
	if err := securefile.PrepareDirectory(workspace, 0o700); err != nil {
		return false, err
	}
	workInfo, err := os.Stat(workspace)
	if err != nil {
		return false, err
	}
	if workInfo.Mode().Perm() != 0o700 {
		return false, fmt.Errorf("identity workspace must be private")
	}
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		if err := os.Remove(filepath.Join(workspace, "identity.sqlite"+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	work, err := os.OpenFile(filepath.Join(workspace, "identity.sqlite"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, err
	}
	defer os.Remove(work.Name())
	defer work.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(work, hash), io.LimitReader(source, maxDatabaseSize+1))
	if err != nil {
		return false, err
	}
	if n > maxDatabaseSize {
		return false, fmt.Errorf("headscale database exceeds the snapshot limit")
	}
	if err := work.Close(); err != nil {
		return false, err
	}
	if err := updateUsers(ctx, work.Name(), bindings, restore); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := checkSidecars(root); err != nil {
		return false, err
	}
	current, err := root.Lstat(databaseName)
	if err != nil {
		return false, err
	}
	if !os.SameFile(current, info) {
		return false, fmt.Errorf("headscale database was replaced during takeover")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	check := sha256.New()
	if _, err := io.Copy(check, io.LimitReader(source, maxDatabaseSize+1)); err != nil {
		return false, err
	}
	if string(check.Sum(nil)) != string(hash.Sum(nil)) {
		return false, fmt.Errorf("headscale database changed during takeover")
	}
	return install(root, work.Name(), info)
}
