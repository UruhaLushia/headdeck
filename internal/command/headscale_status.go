package command

import (
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/spf13/cobra"
)

func newHeadscaleStatusCommand(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the Headscale systemd service state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			control, err := c.HeadscaleService(cmd.Context())
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(control)
			}
			t := newTable(rt.Stdout)
			t.header("FIELD", "VALUE")
			t.row("unit", control.Unit)
			t.row("driver", control.Driver)
			if state := control.State; state != nil {
				t.row("load", state.LoadState)
				t.row("state", state.ActiveState)
				t.row("substate", state.SubState)
				t.row("uptime", dash(state.Uptime))
				t.row("resident memory", protocol.FormatMemory(state.RSSBytes))
				t.row("reload supported", yesNo(state.CanReload))
			}
			t.flush()
			return nil
		},
	}
}
