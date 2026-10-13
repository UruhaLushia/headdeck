package tui

import (
	"errors"

	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (m *Model) clearEditor() {
	m.editKey, m.editValue, m.editCursor = "", "", 0
	m.resource = nil
	m.objectForm = nil
	m.settingPopup = nil
	m.confirmation = nil
	m.focus = focusContent
}

func (m *Model) formError() string {
	var status *client.StatusError
	var problem *protocol.Error
	if errors.As(m.err, &status) {
		problem = status.Proto
	} else {
		errors.As(m.err, &problem)
	}
	if problem != nil {
		if problem.Code == protocol.CodeClientConfig {
			return m.tr("Invalid value. Check the format and try again.")
		}
		return m.tr(problem.Message)
	}
	return m.err.Error()
}
