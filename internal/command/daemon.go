package command

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/daemon"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func newDaemonCommand(rt *Runtime) *cobra.Command {
	var (
		checkOnly  bool
		printCfg   bool
		socketMode string
	)
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the Headdeck daemon in the foreground",
		Long: "Runs the daemon in the foreground, which is how systemd starts it.\n" +
			"The daemon uses Headscale's Unix management socket and is the only component allowed\n" +
			"to talk to the Headscale API.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for {
				cfg, err := config.Load(rt.ConfigPath)
				if err != nil {
					return configError(err)
				}
				cfg.IPC.Listen = rt.Socket
				if socketMode != "" {
					cfg.IPC.SocketMode = socketMode
				}
				if checkOnly || printCfg {
					return runDaemonCheck(rt, cfg, printCfg)
				}
				checks := cfg.Check()
				if config.ChecksFailed(checks) {
					reportChecks(rt, checks)
					return protocol.NewError(protocol.CodeClientConfig,
						"daemon configuration failed validation")
				}
				d, err := daemon.New(daemon.Options{
					Config:        cfg,
					ConfigPath:    rt.ConfigPath,
					Authorization: &grants.Store{Path: rt.Authorization},
					LogLevel:      cfg.LogLevel(),
					LogJSON:       cfg.LogJSONEnabled(),
				})
				if err != nil {
					return err
				}
				err = d.Run(cmd.Context())
				if !errors.Is(err, daemon.ErrReload) {
					return err
				}
			}
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "validate configuration and exit")
	cmd.Flags().BoolVar(&printCfg, "print-config", false, "print the effective configuration and exit")
	cmd.Flags().StringVar(&socketMode, "socket-mode", "", "override ipc.socket_mode (octal)")
	return cmd
}

func runDaemonCheck(rt *Runtime, cfg *config.Config, printCfg bool) error {
	if printCfg && rt.JSON {
		if err := rt.printJSON(config.Settings(cfg)); err != nil {
			return err
		}
	} else if printCfg {
		out, err := config.Effective(cfg)
		if err != nil {
			return err
		}
		fmt.Fprint(rt.Stdout, out)
	}
	checks := cfg.Check()
	if rt.JSON {
		if err := rt.printJSON(checks); err != nil {
			return err
		}
	} else {
		reportChecks(rt, checks)
	}
	if config.ChecksFailed(checks) {
		return protocol.NewError(protocol.CodeClientConfig, "daemon configuration failed validation")
	}
	return nil
}

func reportChecks(rt *Runtime, checks []config.Check) {
	t := newTable(rt.Stdout)
	t.header("CHECK", "STATUS", "DETAIL")
	for _, c := range checks {
		t.row(c.Name, c.Status, dash(c.Message))
	}
	t.flush()
}

func configError(err error) error {
	return protocol.NewError(protocol.CodeClientConfig, "%v", err)
}
