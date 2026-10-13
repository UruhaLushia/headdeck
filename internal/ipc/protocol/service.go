package protocol

const PathHeadscaleService = "/api/v1/system/headscale"

type ServiceAction string

const (
	ServiceStart   ServiceAction = "start"
	ServiceStop    ServiceAction = "stop"
	ServiceRestart ServiceAction = "restart"
	ServiceReload  ServiceAction = "reload"
)

func (a ServiceAction) Valid() bool {
	switch a {
	case ServiceStart, ServiceStop, ServiceRestart, ServiceReload:
		return true
	default:
		return false
	}
}

type ServiceActionRequest struct {
	Reason string `json:"reason,omitempty"`
}

type ServiceUnitState struct {
	RSSBytes    *uint64 `json:"rss_bytes"`
	StartedAt   string  `json:"started_at,omitempty"`
	Uptime      string  `json:"uptime,omitempty"`
	LoadState   string  `json:"load_state"`
	ActiveState string  `json:"active_state"`
	SubState    string  `json:"sub_state"`
	MainPID     uint32  `json:"main_pid"`
	CanReload   bool    `json:"can_reload"`
}
