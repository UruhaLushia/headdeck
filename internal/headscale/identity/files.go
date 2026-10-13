package identity

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

func ownedBy(info os.FileInfo, uid uint32) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && (stat.Uid == 0 || stat.Uid == uid)
}

func checkSidecars(root *os.Root) error {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		info, err := root.Lstat(databaseName + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() != 0 {
			return fmt.Errorf("headscale database has a live or unsafe %s file; stop and checkpoint it first", suffix)
		}
	}
	return nil
}

func install(root *os.Root, path string, info os.FileInfo) (bool, error) {
	const temporary = ".headdeck-identity"
	if err := root.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	out, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, err
	}
	defer root.Remove(temporary)
	defer out.Close()
	in, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer in.Close()
	if _, err := io.Copy(out, in); err != nil {
		return false, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		return false, err
	}
	if err := out.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
		return false, err
	}
	if err := out.Sync(); err != nil {
		return false, err
	}
	if err := out.Close(); err != nil {
		return false, err
	}
	if err := root.Rename(temporary, databaseName); err != nil {
		return false, err
	}
	dir, err := root.Open(".")
	if err != nil {
		return true, err
	}
	defer dir.Close()
	return true, dir.Sync()
}
