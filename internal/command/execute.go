package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func Execute() int {
	rt := NewRuntime()
	root := NewRootCommand(rt)
	root.SetOut(rt.Stdout)
	root.SetErr(rt.Stderr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(rt.Stderr, "canceled")
		return ExitError
	}
	fmt.Fprintln(rt.Stderr, "error:", err)
	return ExitCodeFor(err)
}

func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var statusErr *client.StatusError
	if errors.As(err, &statusErr) {
		return exitCodeForCode(statusErr.Code())
	}
	if protoErr, ok := protocol.AsError(err); ok {
		return exitCodeForCode(protoErr.Code)
	}
	return ExitError
}

func exitCodeForCode(code protocol.ErrorCode) int {
	switch code {
	case "":
		return ExitError
	case protocol.CodeUnauthenticated:
		return ExitAuth
	case protocol.CodePermissionDenied, protocol.CodePeerNotAuthorized:
		return ExitPermission
	case protocol.CodeDaemonUnavailable, protocol.CodeDaemonNotRunning, protocol.CodeRequestTimeout,
		protocol.CodeHeadscaleUnavailable, protocol.CodeHeadscaleUnauthorized:
		return ExitConnection
	case protocol.CodeProtocolIncompatible:
		return ExitProtocol
	case protocol.CodeClientConfig, protocol.CodeHeadscaleNotConfigured:
		return ExitConfig
	default:
		return ExitError
	}
}
