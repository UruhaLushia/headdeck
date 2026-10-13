package web

import "strings"

func agentVersion(agent, token string) string {
	_, rest, found := strings.Cut(agent, token)
	if !found {
		return ""
	}
	end := strings.IndexFunc(rest, func(c rune) bool { return c != '.' && c != '_' && (c < '0' || c > '9') })
	if end >= 0 {
		rest = rest[:end]
	}
	return strings.ReplaceAll(rest, "_", ".")
}

func parseLoginDevice(agent string) loginDevice {
	device := loginDevice{Browser: "Browser"}
	for _, browser := range []struct{ token, name string }{
		{"EdgiOS/", "Edge"}, {"EdgA/", "Edge"}, {"Edg/", "Edge"},
		{"OPR/", "Opera"}, {"SamsungBrowser/", "Samsung Internet"},
		{"FxiOS/", "Firefox"}, {"Firefox/", "Firefox"},
		{"CriOS/", "Chrome"}, {"Chrome/", "Chrome"},
		{"Version/", "Safari"},
	} {
		if version := agentVersion(agent, browser.token); version != "" {
			device.Browser, device.BrowserVersion = browser.name, version
			break
		}
	}
	switch {
	case strings.Contains(agent, "iPhone"), strings.Contains(agent, "iPad"), strings.Contains(agent, "iPod"):
		device.Platform = "iOS"
		device.PlatformVersion = agentVersion(agent, "OS ")
	case strings.Contains(agent, "Android"):
		device.Platform, device.PlatformVersion = "Android", agentVersion(agent, "Android ")
	case strings.Contains(agent, "Windows NT"):
		device.Platform = "Windows"
		version := agentVersion(agent, "Windows NT ")
		device.PlatformVersion = map[string]string{"6.1": "7", "6.2": "8", "6.3": "8.1"}[version]
	case strings.Contains(agent, "CrOS"):
		device.Platform = "ChromeOS"
		parts := strings.Fields(strings.SplitN(agent, "CrOS ", 2)[1])
		if len(parts) > 1 {
			device.PlatformVersion = strings.TrimSuffix(parts[1], ")")
		}
	case strings.Contains(agent, "Mac OS X"):
		device.Platform, device.PlatformVersion = "macOS", agentVersion(agent, "Mac OS X ")
	case strings.Contains(agent, "Linux"):
		device.Platform = "Linux"
	}
	return device
}
