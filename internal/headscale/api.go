package headscale

import (
	"time"
)

type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	Dirty     bool   `json:"dirty"`
}

type User struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"createdAt"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	ProviderID  string    `json:"providerId"`
	Provider    string    `json:"provider"`
}

type PreAuthKey struct {
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	User       *User      `json:"user"`
	Reusable   bool       `json:"reusable"`
	Ephemeral  bool       `json:"ephemeral"`
	Used       bool       `json:"used"`
	ACLTags    []string   `json:"aclTags"`
	Expiration *time.Time `json:"expiration"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Accept both legacy and current Headscale tag fields.
type Node struct {
	ID              string      `json:"id"`
	MachineKey      string      `json:"machineKey"`
	NodeKey         string      `json:"nodeKey"`
	DiscoKey        string      `json:"discoKey"`
	IPAddresses     []string    `json:"ipAddresses"`
	Name            string      `json:"name"`
	GivenName       string      `json:"givenName"`
	User            *User       `json:"user"`
	LastSeen        *time.Time  `json:"lastSeen"`
	Expiry          *time.Time  `json:"expiry"`
	PreAuthKey      *PreAuthKey `json:"preAuthKey"`
	CreatedAt       time.Time   `json:"createdAt"`
	RegisterMethod  string      `json:"registerMethod"`
	Online          bool        `json:"online"`
	ApprovedRoutes  []string    `json:"approvedRoutes"`
	AvailableRoutes []string    `json:"availableRoutes"`
	SubnetRoutes    []string    `json:"subnetRoutes"`
	Tags            []string    `json:"tags"`
	ForcedTags      []string    `json:"forcedTags"`
	ValidTags       []string    `json:"validTags"`
	InvalidTags     []string    `json:"invalidTags"`
}

type CreatePreAuthKeyRequest struct {
	User       string   `json:"user"`
	Reusable   bool     `json:"reusable"`
	Ephemeral  bool     `json:"ephemeral"`
	Expiration string   `json:"expiration,omitempty"`
	ACLTags    []string `json:"aclTags,omitempty"`
}

type APIKey struct {
	ID         string     `json:"id"`
	Prefix     string     `json:"prefix"`
	Expiration *time.Time `json:"expiration"`
	CreatedAt  *time.Time `json:"createdAt"`
	LastSeen   *time.Time `json:"lastSeen"`
}
