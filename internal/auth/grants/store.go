package grants

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
)

type User struct {
	UID  uint32         `json:"uid"`
	Name string         `json:"name"`
	Role principal.Role `json:"role"`
}

type Store struct {
	Path     string
	OwnerUID uint32
}

func (s *Store) Read() ([]User, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	var users []User
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, err
	}
	seen := make(map[uint32]bool)
	for _, u := range users {
		if _, err := principal.ParseRole(string(u.Role)); err != nil {
			return nil, err
		}
		if seen[u.UID] || u.Name == "" {
			return nil, fmt.Errorf("invalid or duplicate authorized uid %d", u.UID)
		}
		seen[u.UID] = true
	}
	return users, nil
}

func (s *Store) Lookup(uid uint32) (*User, error) {
	users, err := s.Read()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.UID == uid {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("uid %s is not authorized; run hdkt auth add <username>", strconv.FormatUint(uint64(uid), 10))
}

// Check ancestors too: replacing a directory can replace the allowlist.
func (s *Store) check() error {
	return s.checkPath(s.Path)
}
func (s *Store) checkPath(start string) error {
	if !filepath.IsAbs(s.Path) {
		return fmt.Errorf("authorization path must be absolute")
	}
	for path := start; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || (stat.Uid != 0 && stat.Uid != s.OwnerUID) || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("authorization path %s has unsafe ownership or permissions", path)
		}
		if path == s.Path && (!info.Mode().IsRegular() || stat.Uid != s.OwnerUID || info.Mode().Perm()&0o007 != 0) {
			return fmt.Errorf("authorization file must be regular and owned by uid %d", s.OwnerUID)
		}
		if path == "/" {
			return nil
		}
	}
}
