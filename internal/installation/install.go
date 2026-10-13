package installation

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/paths"
)

type Options struct {
	AdminUser string
	Role      string
	Unit      string
	Force     bool
	DryRun    bool
}

func Install(ctx context.Context, opts *Options) ([]string, error) {
	if !config.ValidServiceUnit(opts.Unit) {
		return nil, fmt.Errorf("invalid Headscale service unit %q", opts.Unit)
	}
	var steps []string
	record := func(format string, args ...any) {
		steps = append(steps, fmt.Sprintf(format, args...))
	}
	act := func(desc string, fn func() error) error {
		if opts.DryRun {
			record("would %s", desc)
			return nil
		}
		if err := fn(); err != nil {
			return fmt.Errorf("%s: %w", desc, err)
		}
		record("ok: %s", desc)
		return nil
	}

	var uid, gid int
	if err := act("resolve the service account", func() error {
		var err error
		uid, gid, err = serviceAccount()
		return err
	}); err != nil {
		return steps, err
	}
	if err := act("create "+paths.StateDir, func() error {
		dir := filepath.Join(paths.StateDir, "secrets")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		return os.Chown(dir, uid, gid)
	}); err != nil {
		return steps, err
	}
	if err := act("install "+paths.ConfigPath, func() error {
		if _, err := os.Stat(paths.ConfigPath); err == nil && !opts.Force {
			return fmt.Errorf("%s already exists (use --force)", paths.ConfigPath)
		}
		if err := os.MkdirAll(filepath.Dir(paths.ConfigPath), 0o750); err != nil {
			return err
		}
		cfg := config.Default()
		cfg.Features.ServiceUnit = opts.Unit
		if err := writeConfigFile(paths.ConfigPath, cfg, uid, gid); err != nil {
			return err
		}
		return os.Chown(paths.StateDir, uid, gid)
	}); err != nil {
		return steps, err
	}
	if err := act("authorize the administrator user", func() error {
		account, err := user.Lookup(opts.AdminUser)
		if err != nil {
			return err
		}
		adminUID, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			return err
		}
		role, err := principal.ParseRole(opts.Role)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(paths.AuthorizedUsers), 0o750); err != nil {
			return err
		}
		if err := os.Chown(filepath.Dir(paths.AuthorizedUsers), 0, gid); err != nil {
			return err
		}
		store := &grants.Store{Path: paths.AuthorizedUsers}
		return store.Update(gid, func(users []grants.User) ([]grants.User, error) {
			out := make([]grants.User, 0, len(users)+1)
			for _, u := range users {
				if u.UID != uint32(adminUID) {
					out = append(out, u)
				}
			}
			return append(out, grants.User{UID: uint32(adminUID), Name: account.Username, Role: role}), nil
		})
	}); err != nil {
		return steps, err
	}

	if err := act("install the systemd unit", func() error {
		// systemd must be able to read the unit file.
		//nolint:gosec // 0644 is required for a systemd unit
		return os.WriteFile(paths.SystemdUnitDir+"/"+paths.ServiceName,
			[]byte(systemdUnit(opts.Unit)), 0o644)
	}); err != nil {
		return steps, err
	}
	if err := act("reload systemd and enable the service", func() error {
		return Enable(ctx)
	}); err != nil {
		return steps, err
	}
	record("")
	record("Next: configure the public HTTPS URL in hdkt Settings")
	return steps, nil
}
