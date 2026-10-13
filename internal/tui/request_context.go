package tui

import "context"

func (m *Model) requestContext() context.Context {
	if m.requestCtx != nil {
		return m.requestCtx
	}
	return context.Background()
}
