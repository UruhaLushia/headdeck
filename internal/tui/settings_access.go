package tui

func (m *Model) canEditSettings() bool {
	if m.page == pageHeadscale {
		return m.managed != nil && m.managed.Writable && m.can("headscale.config.write")
	}
	if m.page == pageDNS {
		return m.dns != nil && m.dns.Writable && m.can("dns.write")
	}
	return m.can("settings.write")
}
