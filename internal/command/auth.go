package command

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/auth/grants"
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
)

func newAuthCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authorize local Unix users through sudo"}
	for _, action := range []string{"add", "remove", "list"} {
		role := "owner"
		sub := &cobra.Command{Use: action + " [username]", Short: action + " authorized users", Args: cobra.ExactArgs(1)}
		if action == "list" {
			sub.Use = action
			sub.Args = cobra.NoArgs
		}
		sub.RunE = func(cmd *cobra.Command, args []string) error {
			return rt.runAuth(cmd, args, action, role)
		}
		if action == "add" {
			sub.Flags().StringVar(&role, "role", "owner", "pending, member, admin or owner")
		}
		cmd.AddCommand(sub)
	}
	return cmd
}

func (rt *Runtime) runAuth(cmd *cobra.Command, args []string, action, role string) error {
	if os.Geteuid() != 0 {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		sudoArgs := []string{"--", executable, "--users-file", rt.Authorization}
		if rt.JSON {
			sudoArgs = append(sudoArgs, "--json")
		}
		sudoArgs = append(sudoArgs, "auth", action)
		sudoArgs = append(sudoArgs, args...)
		if action == "add" {
			sudoArgs = append(sudoArgs, "--role", role)
		}
		// sudo owns the terminal password prompt; never collect passwords here.
		process := exec.CommandContext(cmd.Context(), "sudo", sudoArgs...)
		process.Stdin, process.Stdout, process.Stderr = rt.Stdin(), rt.Stdout, rt.Stderr
		return process.Run()
	}
	store := &grants.Store{Path: rt.Authorization}
	if action == "list" {
		users, err := store.Read()
		if err != nil {
			return err
		}
		if rt.JSON {
			return rt.printJSON(users)
		}
		t := newTable(rt.Stdout)
		t.header("UID", "USER", "ROLE")
		for _, u := range users {
			t.row(fmt.Sprint(u.UID), u.Name, string(u.Role))
		}
		t.flush()
		return nil
	}
	var uid uint64
	username := args[0]
	if action == "add" {
		account, err := user.Lookup(username)
		if err != nil {
			return err
		}
		uid, err = strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			return err
		}
		username = account.Username
	}
	parsed, err := principal.ParseRole(role)
	if err != nil {
		return err
	}
	err = store.Update(0, func(users []grants.User) ([]grants.User, error) {
		out := make([]grants.User, 0, len(users)+1)
		for _, u := range users {
			if (action == "add" && u.UID != uint32(uid)) || (action == "remove" && u.Name != username) {
				out = append(out, u)
			}
		}
		if action == "add" {
			out = append(out, grants.User{UID: uint32(uid), Name: username, Role: parsed})
		}
		return out, nil
	})
	if err != nil {
		return err
	}
	if rt.JSON {
		return rt.printJSON(map[string]string{"action": action, "user": username})
	}
	fmt.Fprintf(rt.Stdout, "%s: %s\n", action, username)
	return nil
}
