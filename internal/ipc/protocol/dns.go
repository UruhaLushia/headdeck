package protocol

const PathDNS = "/api/v1/dns"

type DNSRecord struct {
	Name  string `json:"name" yaml:"name"`
	Type  string `json:"type" yaml:"type"`
	Value string `json:"value" yaml:"value"`
}

type DNSConfig struct {
	MagicDNS         bool           `json:"magic_dns" yaml:"magic_dns"`
	BaseDomain       string         `json:"base_domain" yaml:"base_domain"`
	OverrideLocalDNS bool           `json:"override_local_dns" yaml:"override_local_dns"`
	Nameservers      DNSNameservers `json:"nameservers" yaml:"nameservers"`
	SearchDomains    []string       `json:"search_domains" yaml:"search_domains"`
	ExtraRecords     []DNSRecord    `json:"extra_records" yaml:"extra_records"`
}

type DNSNameservers struct {
	Global []string            `json:"global" yaml:"global"`
	Split  map[string][]string `json:"split" yaml:"split"`
}

type DNSSettings struct {
	Config          DNSConfig `json:"config"`
	Revision        string    `json:"revision"`
	Writable        bool      `json:"writable"`
	Pending         bool      `json:"pending"`
	RestartRequired bool      `json:"restart_required"`
	ExternalRecords bool      `json:"external_records"`
}

type DNSUpdate struct {
	Config   DNSConfig `json:"config"`
	Revision string    `json:"revision"`
}

type DNSApply struct {
	Revision string `json:"revision"`
}
