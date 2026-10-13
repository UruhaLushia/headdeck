package command

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/spf13/cobra"
)

func newSettingsCommand(rt *Runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "settings", Short: "Manage daemon settings stored in SQLite"}
	get := &cobra.Command{Use: "get [key]", Args: cobra.MaximumNArgs(1), RunE: rt.getSettings}
	var stdin bool
	set := &cobra.Command{Use: "set key [value]", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		var value string
		if stdin {
			if len(args) != 1 {
				return fmt.Errorf("--stdin does not accept a value argument")
			}
			raw, err := io.ReadAll(io.LimitReader(rt.Stdin(), 65537))
			if err != nil {
				return err
			}
			if len(raw) > 65536 {
				return fmt.Errorf("setting is too large")
			}
			value = strings.TrimRight(string(raw), "\r\n")
		} else {
			if len(args) != 2 {
				return fmt.Errorf("a value or --stdin is required")
			}
			if config.SecretSetting(args[0]) {
				return fmt.Errorf("use --stdin to keep secrets out of process arguments")
			}
			value = args[1]
		}
		c, err := rt.Client()
		if err != nil {
			return err
		}
		settings, err := c.UpdateSettings(cmd.Context(), map[string]string{args[0]: value})
		if err != nil {
			return err
		}
		if rt.JSON {
			return rt.printJSON(settings)
		}
		fmt.Fprintln(rt.Stdout, "saved")
		return nil
	}}
	set.Flags().BoolVar(&stdin, "stdin", false, "read the setting value from standard input")
	apply := &cobra.Command{Use: "apply", Short: "Atomically apply a JSON settings object from standard input", Args: cobra.NoArgs, RunE: rt.applySettings}
	cmd.AddCommand(get, set, apply)
	return cmd
}

func (rt *Runtime) applySettings(cmd *cobra.Command, _ []string) error {
	raw, err := io.ReadAll(io.LimitReader(rt.Stdin(), 65537))
	if err != nil {
		return err
	}
	if len(raw) > 65536 {
		return fmt.Errorf("settings are too large")
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("expected a JSON object of setting keys and string values: %w", err)
	}
	if len(values) == 0 {
		return fmt.Errorf("no settings provided")
	}
	c, err := rt.Client()
	if err != nil {
		return err
	}
	settings, err := c.UpdateSettings(cmd.Context(), values)
	if err != nil {
		return err
	}
	if rt.JSON {
		return rt.printJSON(settings)
	}
	fmt.Fprintln(rt.Stdout, "saved")
	return nil
}

func (rt *Runtime) getSettings(cmd *cobra.Command, args []string) error {
	c, err := rt.Client()
	if err != nil {
		return err
	}
	settings, err := c.Settings(cmd.Context())
	if err != nil {
		return err
	}
	if len(args) > 0 {
		filtered := settings.Items[:0]
		for _, item := range settings.Items {
			if item.Key == args[0] {
				filtered = append(filtered, item)
			}
		}
		settings.Items = filtered
		if len(filtered) == 0 {
			return fmt.Errorf("unknown setting %q", args[0])
		}
	}
	if rt.JSON {
		return rt.printJSON(settings)
	}
	t := newTable(rt.Stdout)
	t.header("SETTING", "VALUE")
	for _, item := range settings.Items {
		value := item.Value
		if item.Secret {
			value = "unset"
			if item.Configured {
				value = "[configured]"
			}
		}
		t.row(item.Key, value)
	}
	t.flush()
	if settings.ReloadRequired {
		fmt.Fprintln(rt.Stdout, "settings are reloading")
	}
	return nil
}
