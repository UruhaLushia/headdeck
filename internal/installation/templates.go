package installation

import (
	"fmt"

	"github.com/UruhaLushia/headdeck/internal/paths"
)

func systemdUnit(unit string) string {
	return fmt.Sprintf(`[Unit]
Description=Headdeck management daemon
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
Group=root
Environment=HEADDECK_SERVICE_UNIT=%[3]s
ExecStart=%[1]s daemon --config %[2]s
Restart=on-failure
RestartSec=2s
RuntimeDirectory=headdeck
RuntimeDirectoryMode=0755
StateDirectory=headdeck
StateDirectoryMode=0700
UMask=0077
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
CapabilityBoundingSet=CAP_DAC_OVERRIDE
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
`, paths.BinaryPath, paths.ConfigPath, unit)
}
