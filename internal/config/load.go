package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func (c *Config) LogLevel() string {
	if c.Server.LogLevel != "" {
		return c.Server.LogLevel
	}
	return "info"
}

func (c *Config) SocketFileMode() (os.FileMode, error) {
	raw := strings.TrimSpace(c.IPC.SocketMode)
	if raw == "" {
		return 0o666, nil
	}

	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '7' {
			return 0, fmt.Errorf("ipc.socket_mode %q is not a plain octal mode such as \"0660\"", raw)
		}
	}
	mode, err := strconv.ParseUint(raw, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("ipc.socket_mode %q is not a plain octal mode such as \"0660\"", raw)
	}
	if mode&^0o777 != 0 {
		return 0, fmt.Errorf("ipc.socket_mode %q sets bits outside 0777", raw)
	}
	return os.FileMode(mode), nil
}
