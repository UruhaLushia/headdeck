package tui

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func TestEventWaiterLifecycle(t *testing.T) {
	m := testModel()
	for range 100 {
		m.streamContext, m.streamCancel = context.WithCancel(context.Background())
		m.streamGeneration++
		m.streamCh = make(chan streamMsg, 32)
		m.streamDone = make(chan struct{})
		close(m.streamDone)
		cmd := m.waitForEvent()
		for range 100 {
			if m.waitForEvent() != nil {
				t.Fatal("repeated handshake starts another event waiter")
			}
		}
		result := make(chan tea.Msg, 1)
		go func() { result <- cmd() }()
		m.stopEventStream()
		select {
		case msg := <-result:
			m.streamWaiting = true
			m.Update(msg)
			if !m.streamWaiting {
				t.Fatal("old stream delivery changed the new receiver")
			}
			m.streamWaiting = false
		case <-time.After(time.Second):
			t.Fatal("cancelled event waiter did not exit")
		}
	}
}

func testModel() *Model {
	return &Model{page: pageNodes, conn: connReady, socket: "/run/headdeck/ipc.sock", width: 100, height: 30}
}

func TestConfirmationsDoNotRequireInput(t *testing.T) {
	for _, kind := range []string{"dns.apply", "dns.discard", "service.start", "service.stop", "service.restart", "service.reload", "node.expire", "node.delete", "user.delete"} {
		t.Run(kind, func(t *testing.T) {
			m := testModel()
			action := resourceAction{kind: kind, label: kind, target: "target", confirm: "target"}
			m.beginResource(action)
			if m.confirmation == nil || m.editKey != "" || m.resource != nil {
				t.Fatal("confirmation opened an input form")
			}
			m.confirmationKey(tea.KeyMsg{Type: tea.KeyRight})
			m.confirmationKey(tea.KeyMsg{Type: tea.KeyEnter})
			if m.dialogOpen() || m.resourceBusy {
				t.Fatal("cancel did not close the popup")
			}
			m.beginResource(action)
			_, cmd := m.confirmationKey(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil || !m.resourceBusy {
				t.Fatal("confirmation required additional input")
			}
		})
	}
}

func TestPopupPreservesWideCharacterColumns(t *testing.T) {
	m := testModel()
	m.width = 80
	row := strings.Repeat("中", 40)
	background := strings.Join([]string{row, row, row}, "\n")
	frame := selectionFrame{x: 13, y: 1, width: 54, view: strings.Repeat(" ", 54)}
	for _, line := range strings.Split(m.overlayPopup(background, frame), "\n") {
		if ansi.StringWidth(line) != m.width {
			t.Fatal("popup shifted columns at a wide character boundary")
		}
	}
}

func TestSettingsEditorMasksAndCancelsSecrets(t *testing.T) {
	m := testModel()
	m.hello = &protocol.HelloResponse{Features: []string{"settings.write", "accounts.write"}}
	m.page = pageSettings
	m.settingsGroup = "GitHub login"
	m.settings = &protocol.Settings{Items: []protocol.Setting{
		{Key: "server.web.github.client_secret", Secret: true, Configured: true},
	}}
	m.beginSettingEdit()
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("secret-value")})
	if strings.Contains(m.View(), "secret-value") || !strings.Contains(m.View(), "•") {
		t.Fatal("secret input was not masked")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.editKey != "" || m.editValue != "" || !strings.Contains(m.View(), "[configured]") {
		t.Fatal("cancel did not clear secret input and restore the settings list")
	}
}

func TestSettingsCategories(t *testing.T) {
	for _, width := range []int{40, 120} {
		m := testModel()
		m.width = width
		m.locale = "zh-CN"
		m.page = pageSettings
		m.hello = &protocol.HelloResponse{Features: []string{"settings.write", "headscale.config.write"}}
		m.settings = &protocol.Settings{Items: []protocol.Setting{
			{Key: "features.headscale_service_control", Value: "true"},
			{Key: "server.web.enabled", Value: "true"},
			{Key: "server.web.github.client_id", Value: "github-client"},
		}}
		if m.rowCount() != 3 || m.currentSetting() != nil || strings.Contains(m.settingsView(), "github-client") {
			t.Fatal("category list exposed ungrouped settings")
		}
		if strings.Contains(m.settingsView(), "设置分类") {
			t.Fatal("category list retained its redundant heading")
		}
		if line := strings.Split(m.View(), "\n")[m.layout().listY]; !strings.Contains(ansi.Strip(line), "Web") {
			t.Fatal("category mouse target no longer matches the rendered row")
		}
		m.cursor = 0
		m.press("enter")
		if m.settingsGroup != "Web" || m.rowCount() != 1 || m.currentSetting().Key != "server.web.enabled" {
			t.Fatal("category did not filter the selected setting")
		}
		m.press("enter")
		if m.settingPopup == nil || m.settingPopup.key != "server.web.enabled" {
			t.Fatal("editing used an index from the unfiltered settings")
		}
		m.press("esc")
		m.press("esc")
		if m.settingsGroup != "" || m.cursor != 0 || m.focus != focusContent {
			t.Fatal("back did not restore the selected category")
		}
		m.page = pageHeadscale
		m.cursor = 0
		m.managed = &protocol.ManagedConfig{Writable: true, Items: []protocol.Setting{
			{Key: "hs.prefixes.v4", Value: "100.64.0.0/10"}, {Key: "hs.derp.server.enabled", Value: "false"},
		}}
		m.press("enter")
		if m.managedGroup != "IP allocation" || m.currentSetting().Key != "hs.prefixes.v4" {
			t.Fatal("Headscale configuration did not use categories")
		}
		m.press("left")
		if m.managedGroup != "" || m.focus != focusContent {
			t.Fatal("left did not return to Headscale categories")
		}
		for _, line := range strings.Split(m.View(), "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatal("category view overflows the terminal")
			}
		}
	}
}

func sampleNodes() []protocol.Node {
	return []protocol.Node{
		{ID: "1", GivenName: "server", Name: "server.example", User: protocol.Ref{Name: "admin"},
			IPAddresses: []string{"100.64.0.1"}, Online: true},
		{ID: "2", GivenName: "laptop", Name: "laptop.example", User: protocol.Ref{Name: "admin"},
			IPAddresses: []string{"100.64.0.2"}},
		{ID: "3", GivenName: "phone", Name: "phone.example", User: protocol.Ref{Name: "guest"},
			IPAddresses: []string{"100.64.0.3"}},
	}
}

func TestStaleResponsesAreIgnored(t *testing.T) {
	m := testModel()
	m.nodes = sampleNodes()

	// Simulate a refresh that bumped the generation, then an older response
	// arriving late: it must not overwrite the newer state.
	staleGen := m.nodesGen
	m.nodesGen = staleGen + 5
	next, _ := m.Update(nodesMsg{gen: staleGen, nodes: []protocol.Node{{ID: "99", GivenName: "stale"}}})
	m = next.(*Model)
	if len(m.nodes) != 3 {
		t.Fatalf("stale response replaced the node list: %+v", m.nodes)
	}

	next, _ = m.Update(nodesMsg{gen: m.nodesGen, nodes: []protocol.Node{{ID: "7", GivenName: "fresh"}}})
	m = next.(*Model)
	if len(m.nodes) != 1 || m.nodes[0].ID != "7" {
		t.Fatalf("current response was dropped: %+v", m.nodes)
	}
}

func TestConnectionLossIsSurfaced(t *testing.T) {
	m := testModel()
	next, _ := m.Update(helloMsg{err: errors.New("dial unix: no such file or directory")})
	m = next.(*Model)
	if m.conn != connError {
		t.Fatalf("conn = %v, want connError", m.conn)
	}
	view := m.View()
	for _, want := range []string{"Cannot reach the Headdeck daemon", "/run/headdeck/ipc.sock", "hdkt daemon"} {
		if !strings.Contains(view, want) {
			t.Fatalf("error view does not mention %q:\n%s", want, view)
		}
	}
	if strings.Contains(strings.ToLower(view), "headscale api key") {
		t.Fatal("error view must not suggest direct Headscale access")
	}
}

func TestResyncEventForcesSnapshot(t *testing.T) {
	m := testModel()
	next, cmd := m.Update(eventMsg{resync: true, reason: "history_expired"})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("a resync notice must force a refresh")
	}
	if !m.nodesLoading || !m.usersLoading {
		t.Fatal("a resync notice must refresh both collections")
	}
}

func TestPollingCadenceAndSlowRequests(t *testing.T) {
	m := testModel()
	m.page = pageStatus
	m.streamConnected = true
	for range 14 {
		m.Update(tickMsg{})
	}
	if m.statGen != 0 {
		t.Fatal("overview refreshed before fifteen ticks")
	}
	m.Update(tickMsg{})
	first := [3]int{m.statGen, m.nodesGen, m.usersGen}
	if first != [3]int{1, 0, 0} {
		t.Fatalf("overview did not refresh after fifteen ticks: %v", first)
	}
	for range 15 {
		m.Update(tickMsg{})
	}
	if current := [3]int{m.statGen, m.nodesGen, m.usersGen}; current != first {
		t.Fatalf("slow requests were replaced or overlapped: %v", current)
	}
	m.Update(statusMsg{gen: m.statGen, status: &protocol.Status{}})
	if m.status == nil || m.statusLoading {
		t.Fatal("slow status response was not accepted")
	}
	for range 15 {
		m.Update(tickMsg{})
	}
	if m.statGen != first[0]+1 {
		t.Fatal("overview did not resume polling on the next sampling period")
	}
	for _, page := range navigationPages {
		if page == pageStatus {
			continue
		}
		m = testModel()
		m.page = page
		for range 60 {
			m.Update(tickMsg{})
		}
		if m.nodesGen+m.usersGen+m.keysGen+m.settingsGen+m.dnsGen+m.managedGen+m.policyGen+m.accountsGen+m.providersGen != 0 {
			t.Fatalf("page %v fetched resources on a timer", page)
		}
	}
}

func TestResourceDiffsAndSnapshotOrdering(t *testing.T) {
	m := testModel()
	m.nodes = []protocol.Node{{ID: "1", Name: "keep"}, {ID: "2", Name: "old"}, {ID: "3", Name: "remove"}}
	m.detail = &m.nodes[1]
	m.nodesRevision = 1
	send := func(diff protocol.CollectionDiff[protocol.Node]) tea.Cmd {
		t.Helper()
		data, err := json.Marshal(diff)
		if err != nil {
			t.Fatal(err)
		}
		return m.handleEvent(eventMsg{event: protocol.Event{Type: protocol.EventNodesChanged, Data: data}})
	}
	diff := protocol.CollectionDiff[protocol.Node]{BaseRevision: 1, Revision: 2,
		Upsert: []protocol.Node{{ID: "2", Name: "updated"}, {ID: "4", Name: "new"}}, Removed: []string{"3"}}
	if send(diff) != nil || len(m.nodes) != 3 || m.nodes[0].Name != "keep" || m.detail.Name != "updated" || m.nodesRevision != 2 {
		t.Fatal("diff was not applied locally")
	}
	if send(diff) != nil || len(m.nodes) != 3 {
		t.Fatal("replay duplicated a diff")
	}
	m.Update(nodeDetailMsg{id: "2", revision: 1, node: &protocol.Node{ID: "2", Name: "stale"}})
	if m.detail.Name != "updated" {
		t.Fatal("an older detail response overwrote a newer diff")
	}
	m.Update(nodesMsg{gen: m.nodesGen, revision: 1, nodes: []protocol.Node{{ID: "2", Name: "stale"}}})
	if len(m.nodes) != 3 || m.detail.Name != "updated" {
		t.Fatal("an older snapshot overwrote a newer diff")
	}
	if send(protocol.CollectionDiff[protocol.Node]{BaseRevision: 3, Revision: 4, Removed: []string{"1"}}) == nil {
		t.Fatal("a revision gap did not request a snapshot")
	}
	_, cmd := m.Update(nodesMsg{gen: m.nodesGen, revision: 3, nodes: m.nodes})
	if cmd == nil || !m.nodesLoading {
		t.Fatal("a snapshot behind the gap was accepted as current")
	}
	m.Update(nodesMsg{gen: m.nodesGen, revision: 4, nodes: []protocol.Node{{ID: "4", Name: "new"}}})
	if m.detail != nil || m.nodesRevision != 4 {
		t.Fatal("snapshot did not repair a removed device")
	}
}
