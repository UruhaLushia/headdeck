package server

import (
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"net/http"
	"slices"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func (s *Server) settingsResponse(c *config.Config) protocol.Settings {
	out := protocol.Settings{ReloadRequired: config.SettingsChanged(c, s.svc.Config())}
	for _, v := range config.Settings(c) {
		out.Items = append(out.Items, protocol.Setting{Key: v.Key, Value: v.Value, Secret: v.Secret, Configured: v.Configured})
	}
	return out
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermSettingsRead) {
		return
	}
	c, err := s.settings.Load(r.Context())
	if err != nil {
		writeError(r.Context(), w, protocol.NewError(protocol.CodeInternal, "cannot read settings"))
		return
	}
	response := s.settingsResponse(c)
	p, _ := principal.FromContext(r.Context())
	if !p.Has(principal.PermAccountsRead) {
		response.Items = slices.DeleteFunc(response.Items, func(v protocol.Setting) bool { return policy.OwnerSetting(v.Key) })
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSettingsSet(w http.ResponseWriter, r *http.Request) {
	if !s.guard(r.Context(), w, r, principal.PermSettingsWrite) {
		return
	}
	var req protocol.SettingsUpdate
	if err := decodeJSON(bodyFrom(r.Context()), &req); err != nil {
		writeError(r.Context(), w, err)
		return
	}
	if len(req.Changes) == 0 {
		writeError(r.Context(), w, protocol.NewError(protocol.CodeBadRequest, "no settings provided"))
		return
	}
	p, _ := principal.FromContext(r.Context())
	for key := range req.Changes {
		if policy.OwnerSetting(key) && !p.Has(principal.PermAccountsWrite) {
			writeError(r.Context(), w, protocol.NewError(protocol.CodePermissionDenied, "owner required"))
			return
		}
	}
	c, err := s.settings.UpdateValidated(r.Context(), req.Changes, s.validateConfig)
	if err != nil {
		writeError(r.Context(), w, protocol.NewError(protocol.CodeClientConfig, "%v", err))
		return
	}
	s.audit(r.Context(), "settings.write", "settings", "ok", nil)
	response := s.settingsResponse(c)
	writeJSON(w, http.StatusOK, response)
	if response.ReloadRequired && s.reload != nil {
		http.NewResponseController(w).Flush()
		s.reload()
	}
}
