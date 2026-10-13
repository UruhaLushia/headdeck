package servicecontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/processinfo"
	"github.com/coreos/go-systemd/v22/dbus"
)

func (s *systemdController) State(ctx context.Context) (*protocol.ServiceUnitState, error) {
	if err := checkUnit(s.unit, s.unit); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot connect to system bus: %w", ErrUnavailable, err)
	}
	defer conn.Close()
	props, err := conn.GetUnitPropertiesContext(ctx, s.unit)
	if err != nil {
		return nil, fmt.Errorf("read unit state: %w", err)
	}
	state := &protocol.ServiceUnitState{}
	state.LoadState, _ = props["LoadState"].(string)
	state.ActiveState, _ = props["ActiveState"].(string)
	state.SubState, _ = props["SubState"].(string)
	state.CanReload, _ = props["CanReload"].(bool)
	if state.LoadState == "loaded" {
		service, err := conn.GetUnitTypePropertiesContext(ctx, s.unit, "Service")
		if err != nil {
			return nil, fmt.Errorf("read service state: %w", err)
		}
		state.MainPID, _ = service["MainPID"].(uint32)
		if state.MainPID > 0 {
			state.RSSBytes = processinfo.ResidentMemory(int(state.MainPID))
			if micros, ok := service["ExecMainStartTimestamp"].(uint64); ok && micros > 0 {
				started := time.UnixMicro(int64(micros))
				state.StartedAt = started.UTC().Format(time.RFC3339)
				state.Uptime = max(time.Duration(0), time.Since(started)).Truncate(time.Second).String()
			}
		}
	}
	return state, nil
}
