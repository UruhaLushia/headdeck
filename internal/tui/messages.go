package tui

import (
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type helloMsg struct {
	resp *protocol.HelloResponse
	err  error
}

type statusMsg struct {
	gen    int
	status *protocol.Status
	err    error
}

type nodesMsg struct {
	revision uint64
	gen      int
	nodes    []protocol.Node
	err      error
}

type nodeDetailMsg struct {
	revision uint64
	id       string
	node     *protocol.Node
	err      error
}

type usersMsg struct {
	revision uint64
	gen      int
	users    []protocol.User
	err      error
}
