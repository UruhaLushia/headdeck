package command

import (
	"fmt"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/spf13/cobra"
)

func newHeadscaleLoginCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "login", Short: "Manage Headscale login through Headdeck"}
	for _, action := range []string{"apply", "recover"} {
		cmd.AddCommand(&cobra.Command{
			Use: action, Short: action + " the Headscale login transaction", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				c, err := rt.Client()
				if err != nil {
					return err
				}
				op, err := c.ApplyOIDCTakeover(cmd.Context(), action == "recover")
				if err != nil {
					return err
				}
				final, err := waitForOperation(cmd.Context(), c, op.ID, 6*time.Minute, rt)
				if err != nil {
					return err
				}
				if rt.JSON {
					if err := rt.printJSON(final); err != nil {
						return err
					}
				} else {
					printOperation(rt, *final)
				}
				if final.State != protocol.OperationSucceeded {
					return fmt.Errorf("headscale login %s failed: %s", action, final.Message)
				}
				return nil
			},
		})
	}
	return cmd
}
