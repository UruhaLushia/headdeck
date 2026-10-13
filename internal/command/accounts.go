package command

import (
	"fmt"

	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/spf13/cobra"
)

func newAccountsCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Manage future Web accounts and external identity bindings"}
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List accounts", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := rt.Client()
		if err != nil {
			return err
		}
		out, err := c.Accounts(cmd.Context())
		if err != nil {
			return err
		}
		return rt.printJSON(out)
	}})
	cmd.AddCommand(accountSaveCommand(rt, false), accountSaveCommand(rt, true))
	for _, remove := range []bool{false, true} {
		name := "bind"
		if remove {
			name = "unbind"
		}
		cmd.AddCommand(&cobra.Command{Use: name + " <account-id> <issuer> <subject>", Short: "Manage a verified identity binding", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
			c, err := rt.Client()
			if err != nil {
				return err
			}
			return c.BindIdentity(cmd.Context(), args[0], protocol.Identity{Issuer: args[1], Subject: args[2]}, remove)
		}})
	}
	cmd.AddCommand(&cobra.Command{Use: "identities <account-id>", Short: "List bound identities", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := rt.Client()
		if err != nil {
			return err
		}
		out, err := c.IdentityBindings(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return rt.printJSON(out)
	}})
	return cmd
}

func accountSaveCommand(rt *Runtime, update bool) *cobra.Command {
	var label, role, user string
	var disabled bool
	use := "create <label>"
	if update {
		use = "update <account-id>"
	}
	cmd := &cobra.Command{Use: use, Short: "Save an account (bindings survive name changes)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := rt.Client()
		if err != nil {
			return err
		}
		a := protocol.Account{Label: args[0], Role: policy.RolePending}
		id := ""
		if update {
			id = args[0]
			out, err := c.Accounts(cmd.Context())
			if err != nil {
				return err
			}
			found := false
			for _, item := range out.Accounts {
				if item.ID == id {
					a = item
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("account not found")
			}
		}
		if cmd.Flags().Changed("label") {
			a.Label = label
		}
		if cmd.Flags().Changed("role") {
			a.Role = policy.Role(role)
		}
		if cmd.Flags().Changed("headscale-user") {
			a.HeadscaleUserID = user
		}
		if cmd.Flags().Changed("disabled") {
			a.Disabled = disabled
		}
		out, err := c.SaveAccount(cmd.Context(), id, a)
		if err != nil {
			return err
		}
		return rt.printJSON(out)
	}}
	if update {
		cmd.Flags().StringVar(&label, "label", "", "new account label")
	}
	cmd.Flags().StringVar(&role, "role", "pending", policy.RoleNames())
	cmd.Flags().StringVar(&user, "headscale-user", "", "stable Headscale user ID; empty detaches")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "disable the account")
	return cmd
}
