package command

import (
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/UruhaLushia/headdeck/internal/tui"
	"github.com/UruhaLushia/headdeck/internal/version"
)

const (
	ExitOK         = 0
	ExitError      = 1
	ExitAuth       = 2
	ExitConnection = 3
	ExitPermission = 4
	ExitConfig     = 5
	ExitProtocol   = 6
)

type Runtime struct {
	Socket        string
	ConfigPath    string
	Authorization string
	Timeout       time.Duration
	JSON          bool
	Verbose       bool
	Language      string

	StdinReader io.Reader
	Stdout      io.Writer
	Stderr      io.Writer

	client *client.Client
}

func NewRuntime() *Runtime {
	return &Runtime{
		Socket:        paths.SocketPathFor(),
		ConfigPath:    paths.ConfigPathFor(),
		Authorization: paths.AuthorizedUsers,
		Timeout:       10 * time.Second,
		Language:      "auto",
		StdinReader:   os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
	}
}

func NewRootCommand(rt *Runtime) *cobra.Command {
	root := &cobra.Command{
		Use:   "hdkt",
		Short: "Headdeck: TUI-first management for Headscale",
		Long: "hdkt is the Headdeck single binary. It runs the management daemon, the\n" +
			"terminal UI and the CLI. Every management operation is sent to the daemon\n" +
			"over the local IPC socket; there is no direct Headscale mode.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Get().Line(),

		RunE: func(cmd *cobra.Command, _ []string) error {
			return tui.Run(cmd.Context(), tui.Options{
				Language:      rt.Language,
				Socket:        rt.Socket,
				Authorization: rt.Authorization,
				Timeout:       rt.Timeout,
				Out:           rt.Stdout,
			})
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")

	pf := root.PersistentFlags()
	pf.StringVar(&rt.Language, "lang", rt.Language, "TUI language: auto, en or zh-CN (auto follows HDKT_LANG and system locale)")
	pf.StringVar(&rt.Socket, "socket", rt.Socket, "daemon IPC socket path")
	pf.StringVar(&rt.ConfigPath, "config", rt.ConfigPath, "daemon SQLite settings database")
	pf.StringVar(&rt.Authorization, "users-file", rt.Authorization, "root-controlled user authorization file")
	pf.DurationVar(&rt.Timeout, "timeout", rt.Timeout, "per-request timeout")
	pf.BoolVar(&rt.JSON, "json", false, "emit JSON instead of tables")
	pf.BoolVar(&rt.Verbose, "verbose", false, "verbose diagnostics on stderr")

	root.AddCommand(
		newTUICommand(rt),
		newDaemonCommand(rt),
		newStatusCommand(rt),
		newNodesCommand(rt),
		newUsersCommand(rt),
		newAccountsCommand(rt),
		newAuthCommand(rt),
		newSettingsCommand(rt),
		newServiceCommand(rt),
		newHeadscaleCommand(rt),
		newOperationsCommand(rt),
		newRoutesCommand(rt),
		newKeysCommand(rt),
		newVersionCommand(rt),
		newDocsCommand(rt),
	)
	return root
}
