package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type eventMsg struct {
	event protocol.Event

	resync bool
	reason string
}

type streamClosedMsg struct {
	attempt int
}

type streamReadyMsg struct{}

type streamMsg interface{ streamMessage() }

func (eventMsg) streamMessage()        {}
func (streamClosedMsg) streamMessage() {}
func (streamReadyMsg) streamMessage()  {}

const (
	eventDebounce   = 250 * time.Millisecond
	maxEventAttempt = 8
)

func (m *Model) startEventStream() {
	if m.client == nil || m.streamCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.streamContext = ctx
	m.streamGeneration++
	m.streamCh = make(chan streamMsg, 32)
	m.streamDone = make(chan struct{})
	go watchEventStream(ctx, m.client, m.streamCh, m.streamDone, m.lastEventID)
}

func watchEventStream(ctx context.Context, c *client.Client, out chan<- streamMsg, done chan<- struct{}, lastID uint64) {
	defer close(done)
	attempt := 0
	for {
		if delay := client.WatchBackoff(attempt); delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		connected := false
		_ = c.WatchEvents(ctx, client.WatchOptions{
			LastEventID: lastID,
			OnCursor: func(id uint64) {
				lastID = id
				if !connected {
					connected = true
					select {
					case out <- streamReadyMsg{}:
					case <-ctx.Done():
					}
				}
			},
			OnEvent: func(ev protocol.Event) error {
				attempt = 0
				lastID = ev.ID
				select {
				case out <- eventMsg{event: ev}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			},
			OnResync: func(reason string) {
				select {
				case out <- eventMsg{resync: true, reason: reason}:
				case <-ctx.Done():
				}
			},
		})
		if ctx.Err() != nil {
			return
		}
		attempt++
		if attempt > maxEventAttempt {
			attempt = maxEventAttempt
		}
		select {
		case out <- streamClosedMsg{attempt: attempt}:
		case <-ctx.Done():
			return
		}
	}
}

func (m *Model) stopEventStream() {
	m.streamConnected = false
	if m.streamCancel != nil {
		m.streamCancel()
		<-m.streamDone
		m.streamCancel = nil
		m.streamContext = nil
		m.streamWaiting = false
		m.streamGeneration++
		for {
			select {
			case <-m.streamCh:
			default:
				return
			}
		}
	}
}

type streamDelivery struct {
	generation uint64
	message    streamMsg
}

func (m *Model) waitForEvent() tea.Cmd {
	if m.streamContext == nil || m.streamWaiting {
		return nil
	}
	m.streamWaiting = true
	ctx, ch, generation := m.streamContext, m.streamCh, m.streamGeneration
	return func() tea.Msg {
		select {
		case msg := <-ch:
			return streamDelivery{generation: generation, message: msg}
		case <-ctx.Done():
			return streamDelivery{generation: generation}
		}
	}
}

func (m *Model) handleEvent(msg eventMsg) tea.Cmd {
	if msg.resync {
		m.lastEventID = 0
		m.nodesGen++
		m.usersGen++
		m.nodesLoading, m.usersLoading = false, false
		m.nodesRevision, m.usersRevision = 0, 0
		m.nodesWantedRevision, m.usersWantedRevision = 0, 0
		m.pendingEvent = false
		return tea.Batch(m.loadNodes(), m.loadUsers(), m.loadStatus(), m.pageLoad())
	}
	if msg.event.ID > m.lastEventID {
		m.lastEventID = msg.event.ID
	}
	if handled, cmd := m.applyEventDiff(msg.event); handled {
		return cmd
	}
	m.pendingEvent = true
	if m.debounceActive {
		return nil
	}
	return m.eventDebounce()
}

func (m *Model) eventDebounce() tea.Cmd {
	m.debounceActive = true
	return tea.Tick(eventDebounce, func(time.Time) tea.Msg { return eventTickMsg{} })
}

type eventTickMsg struct{}

func (m *Model) handleEventTick() tea.Cmd {
	m.debounceActive = false
	if !m.pendingEvent {
		return nil
	}
	if m.dialogOpen() || m.resourceBusy || m.dnsLoading || m.managedLoading || m.policyLoading || m.keysLoading || m.accountsLoading || m.providersLoading {
		return m.eventDebounce()
	}
	m.pendingEvent = false
	switch m.page {
	case pageStatus:
		return tea.Batch(m.loadStatus(), m.loadKeys())
	case pageNodes, pageNodeDetail:
		return m.loadNodes()
	case pageUsers:
		return m.loadUsers()
	}
	return m.pageLoad()
}
