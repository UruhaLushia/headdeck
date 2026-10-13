package servicecontrol

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

var (
	ErrUnavailable = errors.New("service control is not available on this host")
	ErrWrongUnit   = errors.New("refusing to control a unit other than the configured one")
)

type Controller interface {
	Available() bool
	Driver() string
	Unit() string
	Message() string
	State(ctx context.Context) (*protocol.ServiceUnitState, error)
	Action(ctx context.Context, unit string, action protocol.ServiceAction) (string, error)
}

func New(cfg *config.Config) Controller {
	unit := cfg.Features.ServiceUnit
	if !cfg.Features.HeadscaleServiceControl {
		return &unavailable{unit: unit, reason: "features.headscale_service_control is disabled"}
	}
	switch cfg.Features.ServiceControlDriver {
	case "none":
		return &unavailable{unit: unit, reason: "features.service_control_driver is none"}
	case "systemd":
		return newSystemd(unit)
	default:
		if hasSystemd() {
			return newSystemd(unit)
		}
		return &unavailable{unit: unit, reason: "systemd is not available on this host"}
	}
}

func hasSystemd() bool {
	if os.Getenv("HEADDECK_FORCE_NO_SYSTEMD") != "" {
		return false
	}
	info, err := os.Stat("/run/systemd/system")
	return err == nil && info.IsDir()
}

type unavailable struct {
	unit   string
	reason string
}

func (u *unavailable) Available() bool { return false }

func (u *unavailable) Driver() string { return "none" }

func (u *unavailable) Unit() string { return u.unit }

func (u *unavailable) Message() string {
	return fmt.Sprintf("%s; Headscale API management still works", u.reason)
}

func (u *unavailable) Action(context.Context, string, protocol.ServiceAction) (string, error) {
	return "", fmt.Errorf("%w: %s", ErrUnavailable, u.reason)
}

func (u *unavailable) State(context.Context) (*protocol.ServiceUnitState, error) {
	return nil, fmt.Errorf("%w: %s", ErrUnavailable, u.reason)
}

func checkUnit(configured, requested string) error {
	if requested == "" || requested != configured {
		return fmt.Errorf("%w: configured %q, requested %q", ErrWrongUnit, configured, requested)
	}
	if !config.ValidServiceUnit(requested) {
		return fmt.Errorf("%w: unit name %q is malformed", ErrWrongUnit, requested)
	}
	return nil
}

const jobTimeout = 45 * time.Second
