package grants

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Keep the lock separate from the atomically replaced allowlist.
func (s *Store) Update(gid int, update func([]User) ([]User, error)) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o750); err != nil {
		return err
	}
	if err := s.checkPath(filepath.Dir(s.Path)); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	users, err := s.Read()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	users, err = update(users)
	if err != nil {
		return err
	}
	blob, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".users-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0o640); err != nil {
		return err
	}
	if err := f.Chown(int(s.OwnerUID), gid); err != nil {
		return err
	}
	if _, err := f.Write(append(blob, '\n')); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.Path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
