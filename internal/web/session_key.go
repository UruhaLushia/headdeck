package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/securefile"
)

func loadOrCreateSessionKey(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("a persistent session key file is required")
	}
	if err := securefile.Directory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	info, statErr := os.Lstat(path)
	if statErr == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !securefile.Owned(info) || info.Size() > 4096) {
		return nil, fmt.Errorf("session key must be a regular file with mode 0600")
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		key, derr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if derr != nil || len(key) < 32 {
			return nil, fmt.Errorf("session key %s is not valid base64 or is too short", path)
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		return loadOrCreateSessionKey(path)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := file.Write([]byte(base64.RawStdEncoding.EncodeToString(key) + "\n")); err != nil {
		return nil, fmt.Errorf("write session key %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	return key, nil
}

func ValidateSessionKey(path string) error {
	_, err := loadOrCreateSessionKey(path)
	return err
}
