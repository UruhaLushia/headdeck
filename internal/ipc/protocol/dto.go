package protocol

import "time"

const (
	LocalHost       = "headdeck.local"
	PathPing        = "/ping"
	PathHello       = "/api/v1/system/hello"
	PathStatus      = "/api/v1/system/status"
	PathNodes       = "/api/v1/nodes"
	PathUsers       = "/api/v1/users"
	PathOperations  = "/api/v1/operations"
	PathEvents      = "/api/v1/events"
	PathRoutes      = "/api/v1/routes"
	PathPreAuthKeys = "/api/v1/preauth-keys"
)

// Unauthenticated ping must not disclose resource or version details.
type Ping struct {
	Service string `json:"service"`
}

type HelloRequest struct {
	Client        string   `json:"client"`
	ClientVersion string   `json:"client_version"`
	ProtocolMin   int      `json:"protocol_min"`
	ProtocolMax   int      `json:"protocol_max"`
	Features      []string `json:"features,omitempty"`
}

type HelloResponse struct {
	ServerVersion   string   `json:"server_version"`
	ProtocolVersion int      `json:"protocol_version"`
	ProtocolMin     int      `json:"protocol_min"`
	ProtocolMax     int      `json:"protocol_max"`
	Features        []string `json:"features"`
	Principal       string   `json:"principal"`
	Role            string   `json:"role"`
	ServerTime      string   `json:"server_time"`
	Headscale       *Ref     `json:"headscale,omitempty"`
}

type Ref struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Status struct {
	Server           ServerStatus    `json:"server"`
	Principal        PrincipalStatus `json:"principal"`
	Headscale        HeadscaleStatus `json:"headscale"`
	ServiceControl   ServiceControl  `json:"service_control"`
	Authorization    Authorization   `json:"authorization"`
	ProtocolVersion  int             `json:"protocol_version"`
	CompiledFeatures []string        `json:"compiled_features"`
}

type Diagnostic struct {
	Section     string         `json:"section"`
	CollectedAt time.Time      `json:"collected_at"`
	Format      string         `json:"format"`
	Content     string         `json:"content,omitempty"`
	Samples     []MetricSample `json:"samples,omitempty"`
}

type MetricSample struct {
	Time   time.Time          `json:"time"`
	Values map[string]float64 `json:"values"`
}

type ServerStatus struct {
	State            string  `json:"state"`
	RSSBytes         *uint64 `json:"rss_bytes"`
	Version          string  `json:"version"`
	Commit           string  `json:"commit"`
	BuildTime        string  `json:"build_time"`
	GoVersion        string  `json:"go_version"`
	Platform         string  `json:"platform"`
	StartedAt        string  `json:"started_at"`
	Uptime           string  `json:"uptime"`
	PID              int     `json:"pid"`
	Socket           string  `json:"socket"`
	EventSubscribers int     `json:"event_subscribers"`
}

type PrincipalStatus struct {
	Kind        string   `json:"kind"`
	ID          string   `json:"id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

type HeadscaleStatus struct {
	Configured   bool       `json:"configured"`
	BaseURL      string     `json:"base_url,omitempty"`
	Reachable    bool       `json:"reachable"`
	Version      string     `json:"version,omitempty"`
	Commit       string     `json:"commit,omitempty"`
	APIKeyPrefix string     `json:"api_key_prefix,omitempty"`
	APIKeyExpiry *time.Time `json:"api_key_expiry,omitempty"`
	KeyExpiresIn string     `json:"api_key_expires_in,omitempty"`
	KeyExpired   bool       `json:"api_key_expired,omitempty"`
	LastError    *Error     `json:"last_error,omitempty"`
	CheckedAt    string     `json:"checked_at,omitempty"`
	LatencyMS    int64      `json:"latency_ms,omitempty"`
}

type ServiceControl struct {
	State     *ServiceUnitState `json:"state,omitempty"`
	Supported bool              `json:"supported"`
	Driver    string            `json:"driver,omitempty"`
	Unit      string            `json:"unit,omitempty"`
	Message   string            `json:"message,omitempty"`
}

type Authorization struct {
	Path        string `json:"path"`
	Users       int    `json:"users"`
	MissingFile bool   `json:"missing_file"`
	Error       string `json:"error,omitempty"`
}

// Keep DTOs independent of upstream wire formats.
type Node struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	GivenName       string   `json:"given_name"`
	User            Ref      `json:"user"`
	IPAddresses     []string `json:"ip_addresses"`
	Online          bool     `json:"online"`
	LastSeen        string   `json:"last_seen,omitempty"`
	Expiry          string   `json:"expiry,omitempty"`
	Expired         bool     `json:"expired"`
	RegisterMethod  string   `json:"register_method,omitempty"`
	AvailableRoutes []string `json:"available_routes,omitempty"`
	ApprovedRoutes  []string `json:"approved_routes,omitempty"`
	EnabledRoutes   []string `json:"enabled_routes,omitempty"`
	ForcedTags      []string `json:"forced_tags,omitempty"`
	ValidTags       []string `json:"valid_tags,omitempty"`
	InvalidTags     []string `json:"invalid_tags,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	Ephemeral       bool     `json:"ephemeral"`
	Hostname        string   `json:"hostname,omitempty"`

	OS         string `json:"os,omitempty"`
	Client     string `json:"client,omitempty"`
	ClientVer  string `json:"client_version,omitempty"`
	MachineKey string `json:"machine_key,omitempty"`
	NodeKey    string `json:"node_key,omitempty"`
}

type NodeList struct {
	Nodes    []Node `json:"nodes"`
	Revision uint64 `json:"revision"`
}

type Route struct {
	NodeID    string `json:"node_id"`
	NodeName  string `json:"node_name,omitempty"`
	User      Ref    `json:"user,omitempty"`
	Prefix    string `json:"prefix"`
	Available bool   `json:"available"`
	Approved  bool   `json:"approved"`
	Enabled   bool   `json:"enabled"`
}

type RouteList struct {
	Routes []Route `json:"routes"`
}

// An empty set clears all route approvals.
type SetRoutesRequest struct {
	Routes []string `json:"routes"`
}

// Only creation responses contain usable key material.
type PreAuthKey struct {
	ID         string   `json:"id"`
	Key        string   `json:"key,omitempty"`
	User       Ref      `json:"user"`
	Reusable   bool     `json:"reusable"`
	Ephemeral  bool     `json:"ephemeral"`
	Used       bool     `json:"used"`
	ACLTags    []string `json:"acl_tags,omitempty"`
	Expiration string   `json:"expiration,omitempty"`
	Expired    bool     `json:"expired"`
	CreatedAt  string   `json:"created_at,omitempty"`
}

type PreAuthKeyList struct {
	Keys []PreAuthKey `json:"keys"`
}

type CreatePreAuthKeyRequest struct {
	User       string   `json:"user"`
	Reusable   bool     `json:"reusable"`
	Ephemeral  bool     `json:"ephemeral"`
	Expiration string   `json:"expiration,omitempty"`
	ACLTags    []string `json:"acl_tags,omitempty"`
}

type ExpirePreAuthKeyRequest struct {
	ID string `json:"id"`
}

type ExpireNodeRequest struct {
	ID string `json:"id"`
}

type SetTagsRequest struct {
	Tags []string `json:"tags"`
}

type User struct {
	Provider    string `json:"provider,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	ProviderID  string `json:"provider_identifier,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type UserList struct {
	Users    []User `json:"users"`
	Revision uint64 `json:"revision"`
}

type RegisterNodeRequest struct {
	UserID string `json:"user_id,omitempty"`
	Key    string `json:"key"`
}

type RenameNodeRequest struct {
	NewName string `json:"new_name"`
}

type DeleteNodeRequest struct {
	ConfirmName string `json:"confirm_name,omitempty"`
}

type OperationState string

const (
	OperationPending   OperationState = "pending"
	OperationRunning   OperationState = "running"
	OperationSucceeded OperationState = "succeeded"
	OperationFailed    OperationState = "failed"
	OperationTimeout   OperationState = "timeout"
	OperationUnknown   OperationState = "unknown"
)

// A systemd job completing does not imply Headscale is healthy.
type Operation struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	State       OperationState  `json:"state"`
	Target      string          `json:"target,omitempty"`
	RequestedBy string          `json:"requested_by,omitempty"`
	StartedAt   string          `json:"started_at"`
	FinishedAt  string          `json:"finished_at,omitempty"`
	Message     string          `json:"message,omitempty"`
	Error       *Error          `json:"error,omitempty"`
	Steps       []OperationStep `json:"steps,omitempty"`
}

type OperationStep struct {
	Name    string         `json:"name"`
	State   OperationState `json:"state"`
	At      string         `json:"at"`
	Message string         `json:"message,omitempty"`
	Error   *Error         `json:"error,omitempty"`
}
