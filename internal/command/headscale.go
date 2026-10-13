package command

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func newHeadscaleCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headscale",
		Short: "Manage the Headscale service through the daemon",
		Long: "These operations are performed by the daemon, which holds the only\n" +
			"credential and the only authorization to touch the Headscale unit. They are\n" +
			"unrelated to `hdkt service`, which manages Headdeck itself.",
	}
	cmd.AddCommand(newHeadscaleStatusCommand(rt))
	cmd.AddCommand(newHeadscaleConfigCommand(rt))
	cmd.AddCommand(newHeadscaleLoginCommand(rt))
	for _, action := range []protocol.ServiceAction{protocol.ServiceStart, protocol.ServiceStop, protocol.ServiceRestart, protocol.ServiceReload} {
		cmd.AddCommand(newHeadscaleActionCommand(rt, action))
	}
	return cmd
}

func newHeadscaleActionCommand(rt *Runtime, action protocol.ServiceAction) *cobra.Command {
	var (
		wait    bool
		timeout time.Duration
		reason  string
	)
	cmd := &cobra.Command{
		Use:   string(action),
		Short: string(action) + " the Headscale unit via the daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			op, err := c.HeadscaleAction(ctx, action, reason)
			if err != nil {
				return err
			}
			if rt.JSON && !wait {
				return rt.printJSON(op)
			}
			if !rt.JSON {
				fmt.Fprintf(rt.Stdout, "%s requested: operation %s (target %s)\n", action, op.ID, op.Target)
			}
			if !wait {
				fmt.Fprintf(rt.Stdout, "track it with: hdkt operations show %s\n", op.ID)
				return nil
			}
			final, err := waitForOperation(ctx, c, op.ID, timeout, rt)
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
				return fmt.Errorf("%s did not complete: %s", action, final.State)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for the operation to finish")
	cmd.Flags().DurationVar(&timeout, "wait-timeout", 2*time.Minute, "how long to wait with --wait")
	cmd.Flags().StringVar(&reason, "reason", "", "reason recorded in the audit log")
	return cmd
}

func waitForOperation(ctx context.Context, c *client.Client, id string, timeout time.Duration, rt *Runtime) (*protocol.Operation, error) {
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		op, err := c.Operation(ctx, id)
		if err != nil {
			return nil, err
		}
		if string(op.State) != last {
			last = string(op.State)
			fmt.Fprintf(rt.Stderr, "operation %s: %s %s\n", op.ID, op.State, op.Message)
		}
		switch op.State {
		case protocol.OperationSucceeded, protocol.OperationFailed, protocol.OperationTimeout:
			return op, nil
		}
		if time.Now().After(deadline) {
			return op, fmt.Errorf("timed out after %s waiting for operation %s", timeout, id)
		}
		select {
		case <-ctx.Done():
			return op, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func printOperation(rt *Runtime, op protocol.Operation) {
	t := newTable(rt.Stdout)
	t.header("FIELD", "VALUE")
	t.row("id", op.ID)
	t.row("kind", op.Kind)
	t.row("target", dash(op.Target))
	t.row("state", string(op.State))
	t.row("requested by", dash(op.RequestedBy))
	t.row("started", formatTime(op.StartedAt))
	if op.FinishedAt != "" {
		t.row("finished", formatTime(op.FinishedAt))
	}
	if op.Message != "" {
		t.row("message", op.Message)
	}
	if op.Error != nil {
		t.row("error", string(op.Error.Code)+": "+op.Error.Message)
	}
	t.flush()
	if len(op.Steps) > 0 {
		fmt.Fprintln(rt.Stdout)
		s := newTable(rt.Stdout)
		s.header("STEP", "STATE", "AT", "DETAIL")
		for _, step := range op.Steps {
			detail := step.Message
			if step.Error != nil {
				detail = string(step.Error.Code) + ": " + step.Error.Message
			}
			s.row(step.Name, string(step.State), formatTime(step.At), dash(detail))
		}
		s.flush()
	}
}

func newOperationsCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operations",
		Short: "Inspect long-running daemon operations",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "show <id>",
			Short: "Show one operation",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := rt.Client()
				if err != nil {
					return err
				}
				op, err := c.Operation(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if rt.JSON {
					return rt.printJSON(op)
				}
				printOperation(rt, *op)
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List recent operations",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				c, err := rt.Client()
				if err != nil {
					return err
				}
				ops, err := c.Operations(cmd.Context())
				if err != nil {
					return err
				}
				if rt.JSON {
					return rt.printJSON(ops)
				}
				if len(ops.Operations) == 0 {
					fmt.Fprintln(rt.Stdout, "no operations recorded")
					return nil
				}
				t := newTable(rt.Stdout)
				t.header("ID", "KIND", "TARGET", "STATE", "STARTED", "MESSAGE")
				for _, op := range ops.Operations {
					t.row(op.ID, op.Kind, dash(op.Target), string(op.State),
						formatTime(op.StartedAt), dash(op.Message))
				}
				t.flush()
				return nil
			},
		},
	)
	return cmd
}
