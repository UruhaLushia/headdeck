package web

import (
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"net/http"
	"slices"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) settingsResponse(c *config.Config) protocol.Settings {
	out := protocol.Settings{Items: []protocol.Setting{}, ReloadRequired: config.SettingsChanged(c, s.svc.Config())}
	for _, v := range config.Settings(c) {
		out.Items = append(out.Items, protocol.Setting{Key: v.Key, Value: v.Value, Secret: v.Secret, Configured: v.Configured})
	}
	return out
}

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	s.endpoint(mux, "GET /api/settings", []principal.Permission{principal.PermSettingsRead}, func(r *http.Request) (any, error) {
		out, err := s.settings.Load(r.Context())
		if err != nil {
			return nil, err
		}
		response := s.settingsResponse(out)
		p, _ := principalFrom(r)
		if !p.Has(principal.PermAccountsRead) {
			response.Items = slices.DeleteFunc(response.Items, func(v protocol.Setting) bool { return policy.OwnerSetting(v.Key) })
		}
		return response, nil
	})
	s.endpoint(mux, "PATCH /api/settings", []principal.Permission{principal.PermSettingsWrite}, withBody(func(r *http.Request, v protocol.SettingsUpdate) (any, error) {
		if len(v.Changes) == 0 {
			return nil, protocol.NewError(protocol.CodeBadRequest, "no settings provided")
		}
		p, _ := principalFrom(r)
		for key := range v.Changes {
			if policy.OwnerSetting(key) && !p.Has(principal.PermAccountsWrite) {
				return nil, protocol.NewError(protocol.CodePermissionDenied, "owner required")
			}
		}
		out, err := s.settings.UpdateValidated(r.Context(), v.Changes, s.validateConfig)
		if err != nil {
			return nil, protocol.NewError(protocol.CodeClientConfig, "%v", err)
		}
		return s.settingsResponse(out), nil
	}), func(out any) {
		if settings, ok := out.(protocol.Settings); ok && settings.ReloadRequired && s.reload != nil {
			s.reload()
		}
	})
}
