package securefile

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func Owned(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && (stat.Uid == 0 || stat.Uid == uint32(os.Geteuid()))
}

func Directory(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("protected directory must use a clean absolute path")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || !Owned(info) || info.Mode().Perm()&0o022 != 0 && (current == path || info.Mode()&os.ModeSticky == 0) {
			return fmt.Errorf("directory %s has unsafe ownership or permissions", current)
		}
		if current == "/" {
			return nil
		}
	}
}

func PrepareDirectory(path string, mode os.FileMode) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("protected directory must use a clean absolute path")
	}
	parent := path
	for {
		_, err := os.Lstat(parent)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent = filepath.Dir(parent)
	}
	if err := Directory(parent); err != nil {
		return err
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return Directory(path)
}
