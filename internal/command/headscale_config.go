package command

import (
	"fmt"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/installation"
	"github.com/spf13/cobra"
	"os"
)

func newHeadscaleConfigCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Authorize configuration management; edit DNS in the TUI"}
	cmd.AddCommand(&cobra.Command{Use: "authorize", Short: "Back up and authorize Headscale configuration management (requires root)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("run sudo hdkt headscale config authorize")
		}
		cfg, err := config.Load(rt.ConfigPath)
		if err != nil {
			return err
		}
		if err := installation.AuthorizeHeadscaleConfig(cmd.Context(), cfg); err != nil {
			return err
		}
		fmt.Fprintln(rt.Stdout, "configuration authorized; edit DNS in hdkt; Headscale has not been restarted")
		return nil
	}})
	return cmd
}
