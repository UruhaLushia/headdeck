package servicecontrol

import (
	"context"
	"fmt"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/coreos/go-systemd/v22/dbus"
)

type systemdController struct {
	unit string
}

func newSystemd(unit string) Controller { return &systemdController{unit: unit} }

func (s *systemdController) Available() bool { return true }

func (s *systemdController) Driver() string { return "systemd" }

func (s *systemdController) Unit() string { return s.unit }

func (s *systemdController) Message() string { return "" }

func (s *systemdController) Action(ctx context.Context, unit string, action protocol.ServiceAction) (string, error) {
	if !action.Valid() {
		return "", protocol.NewError(protocol.CodeBadRequest, "unknown service action")
	}
	if err := checkUnit(s.unit, unit); err != nil {
		return "", err
	}
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: cannot connect to the system bus: %w", ErrUnavailable, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	result := make(chan string, 1)
	var jobID int
	switch action {
	case protocol.ServiceStart:
		jobID, err = conn.StartUnitContext(ctx, unit, "fail", result)
	case protocol.ServiceStop:
		jobID, err = conn.StopUnitContext(ctx, unit, "fail", result)
	case protocol.ServiceRestart:
		jobID, err = conn.RestartUnitContext(ctx, unit, "fail", result)
	case protocol.ServiceReload:
		jobID, err = conn.ReloadUnitContext(ctx, unit, "fail", result)
	}
	if err != nil {
		return "", fmt.Errorf("systemd refused to %s %s: %w", action, unit, err)
	}
	job := fmt.Sprintf("%s/%d", unit, jobID)
	select {
	case <-ctx.Done():
		return job, fmt.Errorf("waiting for systemd job: %w", ctx.Err())
	case status := <-result:
		if status != "done" {
			return job, fmt.Errorf("systemd job %s finished with %s", job, status)
		}
		return job, nil
	}
}
