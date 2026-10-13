package command

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/installation"
	"github.com/UruhaLushia/headdeck/internal/paths"
)

func newServiceCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Install and inspect the local Headdeck systemd service",
		Long: "These commands manage Headdeck itself on this host. They are unrelated to\n" +
			"`hdkt headscale restart`, which asks the daemon to restart the Headscale\n" +
			"unit.",
	}
	cmd.AddCommand(
		newServiceInstallCommand(rt),
		newServiceUninstallCommand(rt),
		newServiceStatusCommand(rt),
	)
	return cmd
}

func newServiceInstallCommand(rt *Runtime) *cobra.Command {
	opts := &installation.Options{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the daemon service, settings and authorization (requires root)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() != 0 && !opts.DryRun {
				return fmt.Errorf("service install must run as root (try: sudo hdkt service install ...)")
			}
			if opts.AdminUser == "" {
				return fmt.Errorf("--admin-user is required")
			}

			if opts.Unit == "" {
				opts.Unit = paths.ServiceUnit
			}
			steps, err := installation.Install(cmd.Context(), opts)
			for _, s := range steps {
				fmt.Fprintln(rt.Stdout, s)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&opts.AdminUser, "admin-user", "", "local user allowed to administer the daemon")
	cmd.Flags().StringVar(&opts.Role, "role", "owner", "role granted to the authorized user")
	cmd.Flags().StringVar(&opts.Unit, "service-unit", paths.ServiceUnit, "Headscale unit the daemon may control")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "print the actions without changing the system")
	return cmd
}

func newServiceUninstallCommand(rt *Runtime) *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the daemon service (requires root)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("service uninstall must run as root")
			}
			if err := installation.Disable(cmd.Context()); err != nil {
				return err
			}
			for _, path := range []string{
				filepath.Join(paths.SystemdUnitDir, paths.ServiceName),
			} {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return err
				}
				fmt.Fprintf(rt.Stdout, "removed %s\n", path)
			}
			if purge {
				for _, path := range []string{paths.ConfigPath, paths.AuthorizedUsers} {
					if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
						return err
					}
					fmt.Fprintf(rt.Stdout, "purged %s\n", path)
				}
			}
			if err := installation.Reload(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(rt.Stdout, "service removed; configuration kept unless --purge was given")
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also remove settings, authorization and local authorization")
	return cmd
}

func newServiceStatusCommand(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the local service state and daemon reachability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			state := installation.State(ctx)
			reachable := rt.Ping(ctx) == nil
			if rt.JSON {
				return rt.printJSON(map[string]any{
					"unit":      paths.ServiceName,
					"state":     state,
					"reachable": reachable,
					"socket":    rt.Socket,
				})
			}
			t := newTable(rt.Stdout)
			t.header("FIELD", "VALUE")
			t.row("unit", paths.ServiceName)
			t.row("systemd state", dash(state))
			t.row("socket", rt.Socket)
			t.row("daemon reachable", yesNo(reachable))
			t.flush()
			if !reachable {
				fmt.Fprintf(rt.Stdout, "\nThe daemon is not answering. Start it with:\n  sudo systemctl start %s\n", paths.ServiceName)
			}
			return nil
		},
	}
}
