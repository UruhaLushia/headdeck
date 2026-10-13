package command

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/paths"
)

func newStatusCommand(rt *Runtime) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon, protocol and Headscale status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			hello, err := rt.Hello(ctx)
			if err != nil {
				return connectionHelp(rt, err)
			}
			c, err := rt.Client()
			if err != nil {
				return err
			}
			status, err := c.Status(ctx, refresh)
			if err != nil {
				return err
			}
			if rt.JSON {
				return rt.printJSON(map[string]any{"hello": hello, "status": status})
			}
			printStatus(rt, hello, status)
			return nil
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "force a fresh Headscale probe")
	return cmd
}

func printStatus(rt *Runtime, hello *protocol.HelloResponse, status *protocol.Status) {
	t := newTable(rt.Stdout)
	t.header("HEADDECK DAEMON", "VALUE")
	t.row("daemon", fmt.Sprintf("%s (%s) pid %d", status.Server.Version, status.Server.Commit, status.Server.PID))
	t.row("daemon state", status.Server.State)
	t.row("resident memory", protocol.FormatMemory(status.Server.RSSBytes))
	t.row("platform", status.Server.Platform)
	t.row("socket", status.Server.Socket)
	t.row("uptime", status.Server.Uptime)
	t.row("protocol", fmt.Sprintf("%d", hello.ProtocolVersion))
	t.row("principal", status.Principal.Kind+":"+status.Principal.ID)
	t.row("role", status.Principal.Role)
	t.row("features", strings.Join(hello.Features, ", "))
	t.row("authorized users", fmt.Sprintf("%d (%s)", status.Authorization.Users, status.Authorization.Path))
	if status.Authorization.MissingFile {
		t.row("authorization file", "missing: no user can connect")
	}
	if status.Authorization.Error != "" {
		t.row("authorization error", status.Authorization.Error)
	}
	t.flush()

	fmt.Fprintln(rt.Stdout)
	h := newTable(rt.Stdout)
	h.header("HEADSCALE", "VALUE")
	h.row("base_url", dash(status.Headscale.BaseURL))
	h.row("reachable", yesNo(status.Headscale.Reachable))
	h.row("version", dash(status.Headscale.Version))
	h.row("api key prefix", dash(status.Headscale.APIKeyPrefix))
	switch {
	case status.Headscale.APIKeyExpiry != nil:
		h.row("api key expires", status.Headscale.APIKeyExpiry.Local().Format("2006-01-02 15:04:05")+
			" ("+status.Headscale.KeyExpiresIn+")")
	case status.Headscale.Reachable:
		h.row("api key expires", "not reported by Headscale")
	}
	if status.Headscale.LatencyMS > 0 {
		h.row("latency", fmt.Sprintf("%d ms", status.Headscale.LatencyMS))
	}
	if status.Headscale.LastError != nil {
		h.row("last error", string(status.Headscale.LastError.Code)+": "+status.Headscale.LastError.Message)
	}
	h.row("service control", serviceControlLine(status.ServiceControl))
	if state := status.ServiceControl.State; state != nil {
		h.row("service state", state.ActiveState+" / "+state.SubState)
		h.row("uptime", dash(state.Uptime))
		h.row("resident memory", protocol.FormatMemory(state.RSSBytes))
	}
	h.flush()
}

func serviceControlLine(sc protocol.ServiceControl) string {
	if !sc.Supported {
		if sc.Message != "" {
			return "unavailable: " + sc.Message
		}
		return "unavailable"
	}
	return fmt.Sprintf("%s (%s)", sc.Driver, sc.Unit)
}

func connectionHelp(rt *Runtime, err error) error {
	code := errorCodeOf(err)
	if code != protocol.CodeDaemonUnavailable && code != protocol.CodeDaemonNotRunning &&
		code != protocol.CodeRequestTimeout {
		return err
	}
	fmt.Fprintf(rt.Stderr, `cannot reach the Headdeck daemon.

  socket:   %s
  user:     uid %d

Start it with one of:
  hdkt daemon --config %s      (foreground)
  sudo systemctl start %s

Validate the daemon configuration with:
  hdkt daemon --check --config %s
`, rt.Socket, os.Getuid(), rt.ConfigPath, paths.ServiceName, rt.ConfigPath)
	return err
}

func errorCodeOf(err error) protocol.ErrorCode {
	var statusErr *client.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.Code()
	}
	if protoErr, ok := protocol.AsError(err); ok {
		return protoErr.Code
	}
	return ""
}
