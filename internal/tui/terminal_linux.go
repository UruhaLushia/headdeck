//go:build linux

package tui

import (
	"io"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

func terminalInput(output io.Writer) (io.Reader, func()) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, func() {}
	}
	return tty, func() {
		defer tty.Close()
		state, err := term.MakeRaw(tty.Fd())
		if err != nil {
			return
		}
		defer term.Restore(tty.Fd(), state)
		if _, err := io.WriteString(output, "\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?1015l"); err != nil {
			return
		}
		deadline := time.Now().Add(150 * time.Millisecond)
		events := []unix.PollFd{{Fd: int32(tty.Fd()), Events: unix.POLLIN}}
		buffer := make([]byte, 4096)
		for time.Now().Before(deadline) {
			count, err := unix.Poll(events, 25)
			if err == unix.EINTR {
				continue
			}
			if err != nil || count == 0 || events[0].Revents&unix.POLLIN == 0 {
				break
			}
			if _, err := tty.Read(buffer); err != nil {
				break
			}
		}
		_ = unix.IoctlSetInt(int(tty.Fd()), unix.TCFLSH, unix.TCIFLUSH)
	}
}
