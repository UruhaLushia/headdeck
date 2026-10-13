//go:build !linux

package tui

import "io"

func terminalInput(io.Writer) (io.Reader, func()) { return nil, func() {} }
