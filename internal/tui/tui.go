package tui

import (
	"context"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UruhaLushia/headdeck/internal/i18n"
	"github.com/UruhaLushia/headdeck/internal/ipc/client"
)

type Options struct {
	Language      string
	Socket        string
	Authorization string
	Timeout       time.Duration
	Out           io.Writer
	Headless      bool
}

func Run(ctx context.Context, opts Options) error {
	if _, err := i18n.Resolve(opts.Language); err != nil {
		return err
	}
	c, err := client.New(client.Options{
		SocketPath: opts.Socket,
		Timeout:    opts.Timeout,
		ClientName: "hdkt-tui",
		UserAgent:  "hdkt-tui",
	})
	if err != nil {
		return err
	}
	defer c.Close()

	model := NewModel(c, opts)
	model.requestCtx, model.cancelRequests = context.WithCancel(ctx)
	defer model.cancelRequests()
	defer model.stopEventStream()
	if opts.Headless {
		return nil
	}
	output := opts.Out
	if output == nil {
		output = os.Stdout
	}
	input, cleanup := terminalInput(output)
	defer cleanup()
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithContext(ctx), tea.WithOutput(output)}
	if input != nil {
		options = append(options, tea.WithInput(input))
	}
	program := tea.NewProgram(model, options...)
	_, err = program.Run()
	return err
}

func (m *Model) quit() tea.Cmd {
	m.quitting = true
	if m.cancelRequests != nil {
		m.cancelRequests()
	}
	m.clearEditor()
	m.stopEventStream()
	// Keep the input reader alive until in-flight mouse reports have arrived.
	return tea.Sequence(tea.DisableMouse, tea.Tick(180*time.Millisecond, func(time.Time) tea.Msg { return tea.Quit() }))
}
