package command

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func newRoutesCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "routes",
		Aliases: []string{"route"},
		Short:   "Inspect and approve subnet routes",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List routes advertised by nodes",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				c, err := rt.Client()
				if err != nil {
					return err
				}
				list, err := c.Routes(cmd.Context())
				if err != nil {
					return err
				}
				if rt.JSON {
					return rt.printJSON(list)
				}
				printRoutes(rt, list.Routes)
				return nil
			},
		},
		newRoutesShowCommand(rt),
		newRoutesApproveCommand(rt),
	)
	return cmd
}

func printRoutes(rt *Runtime, routes []protocol.Route) {
	if len(routes) == 0 {
		return
	}
	t := newTable(rt.Stdout)
	t.header("NODE", "NAME", "USER", "PREFIX", "AVAILABLE", "APPROVED", "ENABLED")
	for _, r := range routes {
		t.row(r.NodeID, dash(r.NodeName), dash(r.User.Name), r.Prefix,
			yesNo(r.Available), yesNo(r.Approved), yesNo(r.Enabled))
	}
	t.flush()
}

func newRoutesShowCommand(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show <node-id>",
		Short: "Show the routes of one node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			list, err := c.NodeRoutes(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(list)
			}
			if len(list.Routes) == 0 {
				fmt.Fprintf(rt.Stdout, "node %s advertises no routes\n", args[0])
				return nil
			}
			printRoutes(rt, list.Routes)
			return nil
		},
	}
}

func newRoutesApproveCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "approve <node-id> [prefix...]",
		Short: "Replace the approved route set of a node",
		Long: "Headscale has no additive route API, so this replaces the whole approved\n" +
			"set. With no prefixes it clears every approval.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			prefixes := args[1:]
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if !yes {
				action := "clear all approved routes"
				if len(prefixes) > 0 {
					action = "approve " + strings.Join(prefixes, ", ")
				}
				ok, err := confirm(rt, fmt.Sprintf("node %s: %s?", id, action))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(rt.Stdout, "aborted")
					return nil
				}
			}
			node, err := c.SetNodeRoutes(ctx, id, prefixes)
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(node)
			}
			if len(node.ApprovedRoutes) == 0 {
				fmt.Fprintf(rt.Stdout, "node %s has no approved routes\n", node.ID)
				return nil
			}
			fmt.Fprintf(rt.Stdout, "node %s approved routes: %s\n",
				node.ID, strings.Join(node.ApprovedRoutes, ", "))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newKeysCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keys",
		Aliases: []string{"preauth-keys", "preauthkeys"},
		Short:   "Manage Headscale pre-auth keys",
	}
	cmd.AddCommand(
		newKeysListCommand(rt),
		newKeysCreateCommand(rt),
		newKeysExpireCommand(rt),
		newKeysDeleteCommand(rt),
	)
	return cmd
}

func newKeysListCommand(rt *Runtime) *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pre-auth keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			list, err := c.PreAuthKeys(cmd.Context(), user)
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(list)
			}
			printPreAuthKeys(rt, list.Keys)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "only keys owned by this user")
	return cmd
}

func printPreAuthKeys(rt *Runtime, keys []protocol.PreAuthKey) {
	if len(keys) == 0 {
		fmt.Fprintln(rt.Stdout, "no pre-auth keys")
		return
	}
	t := newTable(rt.Stdout)
	t.header("ID", "USER", "KEY", "REUSABLE", "EPHEMERAL", "USED", "EXPIRES", "TAGS")
	for _, k := range keys {
		expiry := formatTime(k.Expiration)
		if k.Expired {
			expiry += " (expired)"
		}
		t.row(k.ID, dash(k.User.Name), dash(k.Key), yesNo(k.Reusable), yesNo(k.Ephemeral),
			yesNo(k.Used), expiry, dash(strings.Join(k.ACLTags, ",")))
	}
	t.flush()
}

func newKeysCreateCommand(rt *Runtime) *cobra.Command {
	var (
		user       string
		expiration string
		reusable   bool
		ephemeral  bool
		tags       []string
	)
	cmd := &cobra.Command{
		Use:   "create --user <name>",
		Short: "Create a pre-auth key",
		Long: "Prints the key material once. Headscale never returns it again, so store\n" +
			"it where the device that will use it can read it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			key, err := c.CreatePreAuthKey(cmd.Context(), protocol.CreatePreAuthKeyRequest{
				User:       user,
				Reusable:   reusable,
				Ephemeral:  ephemeral,
				Expiration: expiration,
				ACLTags:    tags,
			})
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(key)
			}
			fmt.Fprintf(rt.Stdout, "%s\n", key.Key)
			fmt.Fprintf(rt.Stderr, "id %s, user %s, expires %s\n",
				key.ID, dash(key.User.Name), formatTime(key.Expiration))
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "owner of the key (required)")
	cmd.Flags().StringVar(&expiration, "expiration", "24h", "lifetime such as 24h or 90d, or an RFC3339 timestamp")
	cmd.Flags().BoolVar(&reusable, "reusable", false, "the key can register more than one node")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", false, "nodes registered with this key are ephemeral")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "ACL tag to apply, for example tag:server")
	return cmd
}

func newKeysExpireCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "expire <id>",
		Short: "Expire a pre-auth key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateKey(rt, cmd, args[0], yes, "expire", "expired")
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newKeysDeleteCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a pre-auth key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateKey(rt, cmd, args[0], yes, "delete", "deleted")
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func mutateKey(rt *Runtime, cmd *cobra.Command, id string, yes bool, verb, past string) error {
	c, err := rt.Client()
	if err != nil {
		return err
	}
	if !yes {
		ok, err := confirm(rt, fmt.Sprintf("%s pre-auth key %s?", verb, id))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(rt.Stdout, "aborted")
			return nil
		}
	}
	switch verb {
	case "expire":
		err = c.ExpirePreAuthKey(cmd.Context(), id)
	default:
		err = c.DeletePreAuthKey(cmd.Context(), id)
	}
	if err != nil {
		return err
	}
	if rt.JSON {
		return rt.printJSON(map[string]string{past: id})
	}
	fmt.Fprintf(rt.Stdout, "%s pre-auth key %s\n", past, id)
	return nil
}
