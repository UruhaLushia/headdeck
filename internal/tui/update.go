package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, nil
	}
	switch msg := msg.(type) {
	case providersMsg:
		if msg.gen != m.providersGen {
			return m, nil
		}
		m.providersLoading, m.resourceBusy, m.err = false, false, msg.err
		if msg.err == nil {
			m.providers = msg.providers
			m.clampCursor()
			if msg.saved {
				m.clearEditor()
				m.notice = "Changes saved"
			}
		}
		return m, nil
	case accountsMsg:
		if msg.gen != m.accountsGen {
			return m, nil
		}
		m.accountsLoading, m.resourceBusy, m.err = false, false, msg.err
		if msg.err == nil {
			m.accounts = msg.accounts
			m.clampCursor()
			if msg.saved {
				m.clearEditor()
				m.notice = "Changes saved"
			}
		}
		return m, nil
	case accountSecurityMsg:
		m.resourceBusy, m.err = false, msg.err
		if msg.err == nil && m.page == pageAccounts {
			m.accountSecurity = msg.security
			m.accountMenu()
		}
		return m, nil
	case keysMsg:
		return m.keysUpdated(msg)
	case keyCreatedMsg:
		return m.keyCreated(msg)
	case managedMsg:
		return m.managedUpdated(msg)
	case policyMsg:
		return m.policyUpdated(msg)
	case streamDelivery:
		if msg.generation != m.streamGeneration {
			return m, nil
		}
		m.streamWaiting = false
		return m.Update(msg.message)
	case dnsMsg:
		m.dnsLoading = false
		if msg.gen != m.dnsGen {
			return m, nil
		}
		if msg.saved {
			m.resourceBusy = false
		}
		m.err = msg.err
		if msg.err == nil {
			m.dns = msg.settings
			m.clampCursor()
			if msg.saved {
				m.clearEditor()
				m.notice = "DNS draft saved"
			}
		}
		if msg.err != nil && !msg.saved {
			m.dns = nil
		}
		return m, nil
	case dnsAuthorizedMsg:
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		m.stopEventStream()
		m.conn = connConnecting
		return m, tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return reconnectMsg{} })
	case serviceOperationMsg:
		return m.serviceOperationUpdated(msg)
	case resourceResult:
		return m.resourceUpdated(msg)
	case settingSavedMsg:
		m.resourceBusy = false
		m.err = msg.err
		m.settingsGen++
		if msg.err == nil {
			m.settings = msg.settings
			m.clearEditor()
			m.notice = "Changes saved"
		}
		return m, nil
	case settingsMsg:
		if msg.gen != m.settingsGen {
			return m, nil
		}
		m.err = msg.err
		if msg.err == nil {
			m.settings = msg.settings
			m.clampCursor()
		}
		return m, nil
	case reconnectMsg:
		return m, m.connect()
	case authorizedMsg:
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		return m, m.connect()

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case helloMsg:
		if msg.err != nil {
			m.conn = connError
			m.err = msg.err
			return m, nil
		}
		m.conn = connReady
		m.err = nil
		m.hello = msg.resp
		m.nodesGen++
		m.usersGen++
		m.nodesLoading, m.usersLoading = false, false
		m.nodesRevision, m.usersRevision = 0, 0
		m.nodesWantedRevision, m.usersWantedRevision = 0, 0
		m.startEventStream()
		return m, tea.Batch(m.loadStatus(), m.loadNodes(), m.loadUsers(), m.pageLoad(), m.waitForEvent())

	case eventMsg:
		next := m.handleEvent(msg)
		if next == nil {
			return m, m.waitForEvent()
		}
		return m, tea.Batch(next, m.waitForEvent())

	case streamClosedMsg:
		m.streamConnected = false
		return m, m.waitForEvent()
	case streamReadyMsg:
		m.streamConnected = true
		if m.serviceOperation != nil {
			return m, tea.Batch(m.pollServiceOperation(m.serviceOperation.ID), m.waitForEvent())
		}
		return m, m.waitForEvent()

	case eventTickMsg:
		return m, m.handleEventTick()

	case statusMsg:
		if msg.gen != m.statGen {
			return m, nil
		}
		m.statusLoading = false
		pending := m.statusRefreshPending
		m.statusRefreshPending = false
		if msg.err != nil {
			m.conn = connError
			m.err = msg.err
			return m, nil
		}
		m.conn = connReady
		m.status = msg.status
		m.statusReceivedAt = time.Now()
		if pending {
			return m, m.loadStatus(true)
		}
		return m, nil

	case nodesMsg:
		if msg.gen != m.nodesGen {
			return m, nil
		}
		m.nodesLoading = false
		if msg.err != nil {
			m.err = msg.err
			return m, m.loadStatus(true)
		}
		if !m.dialogOpen() {
			m.err = nil
		}
		if msg.revision >= m.nodesRevision {
			m.nodes, m.nodesRevision = msg.nodes, msg.revision
			m.updateNodeDetail()
		}
		m.clampCursor()
		if m.nodesRevision < m.nodesWantedRevision {
			return m, m.loadNodes()
		}
		return m, nil

	case usersMsg:
		if msg.gen != m.usersGen {
			return m, nil
		}
		m.usersLoading = false
		if msg.err != nil {
			m.err = msg.err
			return m, m.loadStatus(true)
		}
		if !m.dialogOpen() {
			m.err = nil
		}
		if msg.revision >= m.usersRevision {
			m.users, m.usersRevision = msg.users, msg.revision
		}
		m.clampCursor()
		if m.usersRevision < m.usersWantedRevision {
			return m, m.loadUsers()
		}
		return m, nil

	case nodeDetailMsg:
		if m.nodesRevision > msg.revision {
			m.nodeDetailFromCache(msg.id)
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, m.loadStatus(true)
		}
		if !m.dialogOpen() {
			m.err = nil
		}
		m.detail = msg.node
		return m, nil

	case tickMsg:
		m.pollTick++
		if m.conn == connError && m.pollTick%5 == 0 {
			return m, tea.Batch(m.connect(), tickCmd())
		}
		if m.page == pageStatus && m.conn == connReady && !m.statusLoading && m.pollTick%15 == 0 {
			return m, tea.Batch(m.loadStatus(true), tickCmd())
		}
		return m, tickCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

type reconnectMsg struct{}
