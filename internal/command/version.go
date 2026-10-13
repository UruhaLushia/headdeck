package command

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/version"
)

func newVersionCommand(rt *Runtime) *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the hdkt version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := version.Get()
			if rt.JSON {
				return rt.printJSON(info)
			}
			if short {
				fmt.Fprintln(rt.Stdout, info.Version)
				return nil
			}
			fmt.Fprintln(rt.Stdout, info.Line())
			fmt.Fprintf(rt.Stdout, "protocol: %d\n", protocol.ProtocolVersion)
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "print only the version string")
	return cmd
}

func newDocsCommand(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Print the IPC protocol and security reference",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(rt.Stdout, docsText)
			return nil
		},
	}
}
