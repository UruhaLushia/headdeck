package command

import (
	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/tui"
)

func newTUICommand(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Start the terminal UI (also the default with no arguments)",
		Args:  cobra.NoArgs,
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
}
