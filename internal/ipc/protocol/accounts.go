package protocol

import "github.com/UruhaLushia/headdeck/internal/auth/accounts"

const PathAccounts = "/api/v1/accounts"
const PathLoginProviders = "/api/v1/login-providers"

type LoginProvider = accounts.LoginProvider

type Account = accounts.Account
type Identity = accounts.Identity
type IdentityBinding = accounts.Binding
type AccountList struct {
	Accounts []Account `json:"accounts"`
}
type BindingList struct {
	Bindings []IdentityBinding `json:"bindings"`
}
type AccountSecurity struct {
	Account    Account            `json:"account"`
	Password   bool               `json:"password"`
	TOTP       bool               `json:"totp"`
	Identities []IdentityBinding  `json:"identities"`
	Passkeys   []accounts.Passkey `json:"passkeys"`
}
type AccountSecurityAction struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
}
type AccountCredentials struct {
	Account
	Password string `json:"password,omitempty"`
}
