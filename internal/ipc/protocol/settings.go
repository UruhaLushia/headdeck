package protocol

const PathSettings = "/api/v1/settings"

type Setting struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Secret     bool   `json:"secret,omitempty"`
	Configured bool   `json:"configured,omitempty"`
}

type Settings struct {
	Items          []Setting `json:"items"`
	ReloadRequired bool      `json:"reload_required"`
}

type SettingsUpdate struct {
	Changes map[string]string `json:"changes"`
}
