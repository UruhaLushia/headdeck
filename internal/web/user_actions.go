package web

import (
	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"net/http"
)

func (s *Server) userRoutes(mux *http.ServeMux) {
	write := []principal.Permission{principal.PermUsersWrite}
	s.endpoint(mux, "POST /api/users", write, withBody(func(r *http.Request, v protocol.CreateUserRequest) (any, error) {
		return s.svc.CreateUser(r.Context(), v)
	}))
	s.endpoint(mux, "POST /api/users/{id}/rename", write, withBody(func(r *http.Request, v protocol.RenameUserRequest) (any, error) {
		return s.svc.RenameUser(r.Context(), r.PathValue("id"), v.NewName)
	}))
	s.endpoint(mux, "DELETE /api/users/{id}", write, withBody(func(r *http.Request, v protocol.DeleteUserRequest) (any, error) {
		return map[string]bool{"deleted": true}, s.svc.DeleteUser(r.Context(), r.PathValue("id"), v.ConfirmName)
	}))
	keys := []principal.Permission{principal.PermKeysWrite}
	s.endpoint(mux, "POST /api/preauth-keys", keys, withBody(func(r *http.Request, v protocol.CreatePreAuthKeyRequest) (any, error) {
		return s.svc.CreatePreAuthKey(r.Context(), v)
	}))
	s.endpoint(mux, "POST /api/preauth-keys/{id}/expire", keys, func(r *http.Request) (any, error) {
		return map[string]bool{"expired": true}, s.svc.ExpirePreAuthKey(r.Context(), r.PathValue("id"))
	})
	s.endpoint(mux, "DELETE /api/preauth-keys/{id}", keys, func(r *http.Request) (any, error) {
		return map[string]bool{"deleted": true}, s.svc.DeletePreAuthKey(r.Context(), r.PathValue("id"))
	})
	s.endpoint(mux, "GET /api/accounts", []principal.Permission{principal.PermAccountsRead}, func(r *http.Request) (any, error) { return s.svc.Accounts(r.Context()) })
	s.endpoint(mux, "POST /api/accounts", []principal.Permission{principal.PermAccountsWrite}, withBody(func(r *http.Request, v accountMutation) (any, error) {
		return s.svc.SaveAccountCredentials(r.Context(), "", v.Account, v.Password)
	}))
	s.endpoint(mux, "PUT /api/accounts/{id}", []principal.Permission{principal.PermAccountsWrite}, withBody(func(r *http.Request, v protocol.Account) (any, error) {
		return s.svc.SaveAccount(r.Context(), r.PathValue("id"), v)
	}))
	s.endpoint(mux, "GET /api/accounts/{id}/identities", []principal.Permission{principal.PermAccountsRead}, func(r *http.Request) (any, error) { return s.svc.IdentityBindings(r.Context(), r.PathValue("id")) })
	s.endpoint(mux, "GET /api/accounts/{id}/security", []principal.Permission{principal.PermAccountsRead}, func(r *http.Request) (any, error) { return s.svc.AccountSecurity(r.Context(), r.PathValue("id")) })
	s.endpoint(mux, "POST /api/accounts/{id}/security", []principal.Permission{principal.PermAccountsWrite}, withBody(func(r *http.Request, v protocol.AccountSecurityAction) (any, error) {
		return map[string]bool{"updated": true}, s.svc.AccountSecurityAction(r.Context(), r.PathValue("id"), v)
	}))
	for _, method := range []string{"POST", "DELETE"} {
		s.endpoint(mux, method+" /api/accounts/{id}/identities", []principal.Permission{principal.PermAccountsWrite}, withBody(func(r *http.Request, v protocol.Identity) (any, error) {
			return map[string]bool{"updated": true}, s.svc.BindIdentity(r.Context(), r.PathValue("id"), v, r.Method == http.MethodDelete)
		}))
	}
}

type accountMutation struct {
	protocol.Account
	Password string `json:"password"`
}
