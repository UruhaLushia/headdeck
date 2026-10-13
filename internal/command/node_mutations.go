package command

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newNodesRenameCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rename <id> <name>",
		Short: "Rename a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, name := args[0], args[1]
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			current, err := c.Node(ctx, id)
			if err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(rt, fmt.Sprintf("rename node %s (%s) to %q?",
					current.ID, current.GivenName, name))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(rt.Stdout, "aborted")
					return nil
				}
			}
			node, err := c.RenameNode(ctx, id, name)
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(node)
			}
			fmt.Fprintf(rt.Stdout, "renamed node %s to %s\n", node.ID, node.GivenName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newNodesDeleteCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a node (irreversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			node, err := c.Node(ctx, id)
			if err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(rt, fmt.Sprintf(
					"delete node %s (%s, %s)? this cannot be undone",
					node.ID, node.GivenName, strings.Join(node.IPAddresses, ",")))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(rt.Stdout, "aborted")
					return nil
				}
			}
			if err := c.DeleteNode(ctx, id, node.GivenName); err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(map[string]string{"deleted": id})
			}
			fmt.Fprintf(rt.Stdout, "deleted node %s (%s)\n", node.ID, node.GivenName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newNodesExpireCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "expire <id>",
		Short: "Expire (log out) a node without deleting it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			node, err := c.Node(ctx, id)
			if err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(rt, fmt.Sprintf(
					"expire node %s (%s)? the device must re-authenticate to rejoin",
					node.ID, node.GivenName))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(rt.Stdout, "aborted")
					return nil
				}
			}
			updated, err := c.ExpireNode(ctx, id)
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(updated)
			}
			fmt.Fprintf(rt.Stdout, "expired node %s (%s)\n", updated.ID, updated.GivenName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newNodesTagsCommand(rt *Runtime) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "tags <id> [tag...]",
		Short: "Replace the tags of a node",
		Long: "Headscale replaces the whole tag set, so this is not additive. With no tags\n" +
			"it clears every tag.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			tags := args[1:]
			c, err := rt.Client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if !yes {
				action := "clear all tags"
				if len(tags) > 0 {
					action = "set tags " + strings.Join(tags, ", ")
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
			node, err := c.SetNodeTags(ctx, id, tags)
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(node)
			}
			fmt.Fprintf(rt.Stdout, "node %s tags: %s\n", node.ID,
				dash(strings.Join(node.ValidTags, ", ")))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func confirm(rt *Runtime, prompt string) (bool, error) {
	fmt.Fprintf(rt.Stdout, "%s\ntype \"yes\" to continue: ", prompt)
	reader := bufio.NewReader(rt.Stdin())
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes"), nil
}

func newUsersCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "users",
		Aliases: []string{"user"},
		Short:   "Manage Headscale users",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List users",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			list, err := c.Users(cmd.Context())
			if err != nil {
				return connectionHelp(rt, err)
			}
			if rt.JSON {
				return rt.printJSON(list)
			}
			if len(list.Users) == 0 {
				fmt.Fprintln(rt.Stdout, "no users registered")
				return nil
			}
			t := newTable(rt.Stdout)
			t.header("ID", "NAME", "DISPLAY NAME", "EMAIL", "CREATED")
			for _, u := range list.Users {
				t.row(u.ID, u.Name, dash(u.DisplayName), dash(u.Email), formatTime(u.CreatedAt))
			}
			t.flush()
			return nil
		},
	})
	cmd.AddCommand(userMutations(rt)...)
	return cmd
}
