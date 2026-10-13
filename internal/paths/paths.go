package paths

import (
	"os"
)

const (
	SocketPath      = "/run/headdeck/ipc.sock"
	ConfigPath      = "/var/lib/headdeck/settings.db"
	AuthorizedUsers = "/etc/headdeck/users.json"
	ServiceUnit     = "headscale.service"
	ServiceName     = "headdeck.service"
	SystemdUnitDir  = "/etc/systemd/system"
	BinaryPath      = "/usr/local/bin/hdkt"
	StateDir        = "/var/lib/headdeck"
)

func SocketPathFor() string {
	if v := os.Getenv("HEADDECK_SOCKET"); v != "" {
		return v
	}
	return SocketPath
}

func ConfigPathFor() string {
	if v := os.Getenv("HEADDECK_CONFIG"); v != "" {
		return v
	}
	return ConfigPath
}
