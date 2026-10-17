package config

import (
	"time"
)

const (
	DefaultConfigPath  = "/var/lib/headdeck/settings.db"
	DefaultSocketPath  = "/run/headdeck/ipc.sock"
	DefaultServiceUnit = "headscale.service"
)

type Config struct {
	Headscale HeadscaleConfig `json:"headscale"`
	IPC       IPCConfig       `json:"ipc"`
	Server    ServerConfig    `json:"server"`
	Features  FeaturesConfig  `json:"features"`
}

type HeadscaleConfig struct {
	BaseURL            string        `json:"base_url"`
	APIKey             string        `json:"api_key,omitempty"`
	Timeout            time.Duration `json:"timeout"`
	InsecureSkipVerify bool          `json:"insecure_skip_verify"`
	MaxRetries         int           `json:"max_retries"`
}

type IPCConfig struct {
	Listen     string `json:"listen"`
	SocketMode string `json:"socket_mode"`
}

type ServerConfig struct {
	Diagnostics bool      `json:"diagnostics"`
	LogLevel    string    `json:"log_level"`
	LogFormat   string    `json:"log_format"`
	Web         WebConfig `json:"web"`
}

// Web uses separate authentication from local IPC.
type WebConfig struct {
	Enabled        bool   `json:"enabled"`
	PasswordLogin  bool   `json:"password_login"`
	DefaultRole    string `json:"default_role"`
	HeadscaleProxy bool   `json:"headscale_proxy"`
	PublicURL      string `json:"public_url"`

	Listen     string        `json:"listen"`
	SessionTTL time.Duration `json:"session_ttl"`
	// Require Secure outside loopback HTTP.
	CookieSecure bool   `json:"cookie_secure"`
	TLSCert      string `json:"tls_cert"`
	TLSKey       string `json:"tls_key"`
	// Trust X-Forwarded-For only from these CIDRs.
	TrustedProxies []string `json:"trusted_proxies"`
	// Read the 0600 login secret on each login to support rotation.
	TokenFile string `json:"token_file"`
	// Rotating the 0600 session key invalidates all sessions.
	SessionKeyFile string           `json:"session_key_file"`
	OIDC           OIDCConfig       `json:"oidc"`
	GitHub         GitHubConfig     `json:"github"`
	OIDCServer     OIDCServerConfig `json:"oidc_server"`
}

type GitHubConfig struct {
	Enabled           bool   `json:"enabled"`
	ClientID          string `json:"client_id"`
	ClientSecret      string `json:"client_secret"`
	AutoRegister      bool   `json:"auto_register"`
	DisableTokenLogin bool   `json:"disable_token_login"`
}

type OIDCConfig struct {
	Enabled           bool   `json:"enabled"`
	Name              string `json:"name"`
	Issuer            string `json:"issuer"`
	ClientID          string `json:"client_id"`
	ClientSecret      string `json:"client_secret"`
	AutoRegister      bool   `json:"auto_register"`
	DisableTokenLogin bool   `json:"disable_token_login"`
}

type FeaturesConfig struct {
	HeadscaleServiceControl bool   `json:"headscale_service_control"`
	ServiceUnit             string `json:"service_unit"`
	ServiceControlDriver    string `json:"service_control_driver"`
}

func Default() *Config {
	return &Config{
		Headscale: HeadscaleConfig{
			BaseURL:    "http://127.0.0.1:8080",
			Timeout:    8 * time.Second,
			MaxRetries: 2,
		},
		IPC: IPCConfig{
			Listen:     DefaultSocketPath,
			SocketMode: "0666",
		},
		Server: ServerConfig{
			LogLevel:  "info",
			LogFormat: "text",
			//nolint:gosec // the web token and session key fields hold paths, not secrets
			Web: WebConfig{
				Enabled:        false,
				DefaultRole:    "pending",
				HeadscaleProxy: true,
				Listen:         "127.0.0.1:8088",
				SessionTTL:     7 * 24 * time.Hour,
				TokenFile:      "/var/lib/headdeck/secrets/web-token",
				SessionKeyFile: "/var/lib/headdeck/secrets/web-session-key",
				OIDC:           OIDCConfig{Name: "Single sign-on", AutoRegister: true},
				GitHub:         GitHubConfig{AutoRegister: true},
				OIDCServer:     OIDCServerConfig{ClientID: "headscale", SigningKeyFile: "/var/lib/headdeck/secrets/oidc-signing-key.pem", TokenTTL: time.Hour},
			},
		},
		Features: FeaturesConfig{
			HeadscaleServiceControl: true,
			ServiceUnit:             DefaultServiceUnit,
			ServiceControlDriver:    "auto",
		},
	}
}
