package protocol

const (
	PathManagedConfig  = "/api/v1/headscale/config"
	PathHeadscaleLogin = "/api/v1/headscale/login"
	PathPolicy         = "/api/v1/policy"
)

type ManagedConfig struct {
	Items    []Setting `json:"items"`
	Revision string    `json:"revision"`
	Writable bool      `json:"writable"`
	Pending  bool      `json:"pending"`
}

type ManagedConfigUpdate struct {
	Values   map[string]string `json:"values"`
	Revision string            `json:"revision"`
}

type PolicySettings struct {
	Policy   string `json:"policy"`
	Mode     string `json:"mode"`
	Revision string `json:"revision"`
	Writable bool   `json:"writable"`
	Pending  bool   `json:"pending"`
}

type PolicyUpdate struct {
	Policy   string `json:"policy"`
	Revision string `json:"revision"`
}
