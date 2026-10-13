package installation

import (
	"context"
	"os"
	"time"

	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/paths"
)

func State(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := systemConnection(ctx)
	if err != nil {
		return "unknown"
	}
	defer conn.Close()
	props, err := conn.GetUnitPropertiesContext(ctx, paths.ServiceName)
	if err != nil {
		return "unknown"
	}
	state, ok := props["ActiveState"].(string)
	if !ok {
		return "unknown"
	}
	return state
}

func writeConfigFile(path string, cfg *config.Config, uid, gid int) error {
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	return os.Chown(path, uid, gid)
}

func serviceAccount() (int, int, error) { return 0, 0, nil }
