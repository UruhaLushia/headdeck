package installation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/coreos/go-systemd/v22/dbus"
	bus "github.com/godbus/dbus/v5"
)

func systemConnection(ctx context.Context) (*dbus.Conn, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to systemd: %w", err)
	}
	return conn, nil
}

func Enable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	conn, err := systemConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, _, err := conn.EnableUnitFilesContext(ctx, []string{paths.ServiceName}, false, false); err != nil {
		return err
	}
	if err := conn.ReloadContext(ctx); err != nil {
		return err
	}
	return serviceJob(ctx, conn, true)
}

func Disable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	conn, err := systemConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.DisableUnitFilesContext(ctx, []string{paths.ServiceName}, false); err != nil {
		return err
	}
	return serviceJob(ctx, conn, false)
}

func Reload(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := systemConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ReloadContext(ctx)
}

func serviceJob(ctx context.Context, conn *dbus.Conn, start bool) error {
	result := make(chan string, 1)
	var err error
	if start {
		_, err = conn.StartUnitContext(ctx, paths.ServiceName, "replace", result)
	} else {
		_, err = conn.StopUnitContext(ctx, paths.ServiceName, "replace", result)
		var problem bus.Error
		if errors.As(err, &problem) && problem.Name == "org.freedesktop.systemd1.NoSuchUnit" {
			return nil
		}
	}
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case state := <-result:
		if state != "done" {
			return fmt.Errorf("headdeck systemd job finished with %s", state)
		}
		return nil
	}
}
