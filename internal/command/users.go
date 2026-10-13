package command

import (
	"fmt"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/spf13/cobra"
)

func userMutations(rt *Runtime) []*cobra.Command {
	var display, email string
	create := &cobra.Command{Use: "create <name>", Short: "Create a Headscale user", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := rt.Client()
		if err != nil {
			return err
		}
		u, err := c.CreateUser(cmd.Context(), protocol.CreateUserRequest{Name: args[0], DisplayName: display, Email: email})
		if err != nil {
			return err
		}
		return rt.printJSON(u)
	}}
	create.Flags().StringVar(&display, "display-name", "", "display name")
	create.Flags().StringVar(&email, "email", "", "email (not used for login binding)")
	rename := &cobra.Command{Use: "rename <id> <name>", Short: "Rename a local Headscale user", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := rt.Client()
		if err != nil {
			return err
		}
		u, err := c.RenameUser(cmd.Context(), args[0], args[1])
		if err != nil {
			return err
		}
		return rt.printJSON(u)
	}}
	var yes bool
	remove := &cobra.Command{Use: "delete <id>", Short: "Delete an empty, unlinked Headscale user", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := rt.Client()
		if err != nil {
			return err
		}
		users, err := c.Users(cmd.Context())
		if err != nil {
			return err
		}
		for _, u := range users.Users {
			if u.ID != args[0] {
				continue
			}
			if !yes {
				ok, err := confirm(rt, "delete user "+u.Name+"?")
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
			if err := c.DeleteUser(cmd.Context(), u.ID, u.Name); err != nil {
				return err
			}
			fmt.Fprintln(rt.Stdout, "deleted user", u.ID)
			return nil
		}
		return fmt.Errorf("user %s not found", args[0])
	}}
	remove.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	return []*cobra.Command{create, rename, remove}
}
