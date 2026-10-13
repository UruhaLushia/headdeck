package servicecontrol

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

func CheckHeadscaleEnvironment(ctx context.Context, unit string) error {
	if err := checkUnit(unit, unit); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	props, err := conn.GetUnitTypePropertiesContext(ctx, unit, "Service")
	if err != nil {
		return err
	}
	for _, key := range []string{"Environment", "PassEnvironment"} {
		entries, ok := props[key].([]string)
		if !ok {
			return fmt.Errorf("cannot verify Headscale environment configuration")
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry, "HEADSCALE_") {
				return fmt.Errorf("headscale environment configuration overrides are not supported for login takeover")
			}
		}
	}
	files := reflect.ValueOf(props["EnvironmentFiles"])
	if !files.IsValid() || files.Kind() != reflect.Slice || files.Len() != 0 {
		return fmt.Errorf("headscale environment files are not supported for login takeover")
	}
	return nil
}
