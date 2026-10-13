package command

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func newNodesCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "nodes",
		Aliases: []string{"node"},
		Short:   "Inspect and manage Headscale nodes",
	}
	cmd.AddCommand(
		newNodesListCommand(rt),
		newNodesShowCommand(rt),
		newNodesRenameCommand(rt),
		newNodesDeleteCommand(rt),
		newNodesExpireCommand(rt),
		newNodesTagsCommand(rt),
	)
	return cmd
}

func newNodesListCommand(rt *Runtime) *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List nodes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			list, err := c.Nodes(cmd.Context(), user)
			if err != nil {
				return connectionHelp(rt, err)
			}
			if rt.JSON {
				return rt.printJSON(list)
			}
			printNodes(rt, list.Nodes)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "only nodes owned by this Headscale user")
	return cmd
}

func printNodes(rt *Runtime, nodes []protocol.Node) {
	if len(nodes) == 0 {
		fmt.Fprintln(rt.Stdout, "no nodes registered")
		return
	}
	t := newTable(rt.Stdout)
	t.header("ID", "NAME", "USER", "IP ADDRESSES", "STATUS", "LAST SEEN", "EXPIRES")
	for _, n := range nodes {
		status := "offline"
		switch {
		case n.Online:
			status = "online"
		case n.Expired:
			status = "expired"
		}
		t.row(n.ID, dash(n.GivenName), dash(n.User.Name),
			dash(strings.Join(n.IPAddresses, ",")), status,
			formatTime(n.LastSeen), formatTime(n.Expiry))
	}
	t.flush()
}

func newNodesShowCommand(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one node in detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			node, err := c.Node(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(node)
			}
			printNodeDetail(rt, *node)
			return nil
		},
	}
}

func printNodeDetail(rt *Runtime, n protocol.Node) {
	t := newTable(rt.Stdout)
	t.header("FIELD", "VALUE")
	t.row("id", n.ID)
	t.row("given name", dash(n.GivenName))
	t.row("hostname", dash(n.Name))
	t.row("user", dash(n.User.Name))
	t.row("ip addresses", dash(strings.Join(n.IPAddresses, ", ")))
	t.row("status", onOff(n.Online))
	t.row("expired", yesNo(n.Expired))
	t.row("last seen", formatTime(n.LastSeen))
	t.row("expiry", formatTime(n.Expiry))
	t.row("registered", dash(n.RegisterMethod))
	t.row("created", formatTime(n.CreatedAt))
	t.row("ephemeral", yesNo(n.Ephemeral))
	if len(n.AvailableRoutes) > 0 {
		t.row("available routes", strings.Join(n.AvailableRoutes, ", "))
	}
	if len(n.ApprovedRoutes) > 0 {
		t.row("approved routes", strings.Join(n.ApprovedRoutes, ", "))
	}
	if len(n.EnabledRoutes) > 0 {
		t.row("enabled routes", strings.Join(n.EnabledRoutes, ", "))
	}
	if len(n.ValidTags) > 0 {
		t.row("tags", strings.Join(n.ValidTags, ", "))
	}
	if len(n.ForcedTags) > 0 {
		t.row("forced tags", strings.Join(n.ForcedTags, ", "))
	}
	if len(n.InvalidTags) > 0 {
		t.row("invalid tags", strings.Join(n.InvalidTags, ", "))
	}
	t.flush()
}
