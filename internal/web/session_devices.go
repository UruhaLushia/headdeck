package web

import (
	"net/http"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/log"
	"gorm.io/gorm/clause"
)

type loginDevice struct {
	ID              string    `json:"id"`
	Browser         string    `json:"browser"`
	BrowserVersion  string    `json:"browser_version,omitempty"`
	Platform        string    `json:"platform,omitempty"`
	PlatformVersion string    `json:"platform_version,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	Current         bool      `json:"current"`
}

func (s *Server) sessionDevices(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sessions", s.handleSessionDevices)
	mux.HandleFunc("DELETE /api/sessions", s.handleSessionDevices)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleSessionDevices)
}

func (s *Server) handleSessionDevices(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "sign in first"))
		return
	}
	actor, current, ok := s.sessionPrincipal(r, cookie.Value)
	if !ok {
		s.clearCookie(w)
		s.writeError(w, protocol.NewError(protocol.CodeUnauthenticated, "session expired"))
		return
	}
	owner, err := sessionOwner(current.Subject)
	if err != nil {
		s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot read session"))
		return
	}
	if r.Method == http.MethodGet {
		s.maybeRenewSession(w, r, current)
		var records []sessionRecord
		err := s.sessions.orm.WithContext(r.Context()).Where(&sessionRecord{Owner: owner, KeyID: s.sessions.keyID}).Where(clause.Gt{Column: "expires", Value: s.now().Unix()}).Order("created DESC").Order("hash").Find(&records).Error
		if err != nil {
			s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot list sessions"))
			return
		}
		devices := []loginDevice{}
		for _, record := range records {
			item := parseLoginDevice(record.Agent)
			item.ID = record.Hash
			item.CreatedAt = time.Unix(record.Created, 0).UTC()
			item.ExpiresAt = time.Unix(record.Expires, 0).UTC()
			item.Current = record.Hash == sessionHash(cookie.Value)
			devices = append(devices, item)
		}
		s.writeJSON(w, http.StatusOK, devices)
		return
	}
	id := r.PathValue("id")
	filter := sessionRecord{Owner: owner, Hash: id}
	result := s.sessions.orm.WithContext(r.Context()).Where(&filter).Delete(&sessionRecord{})
	if result.Error != nil {
		s.writeError(w, protocol.NewError(protocol.CodeInternal, "cannot revoke sessions"))
		return
	}
	if id != "" && result.RowsAffected == 0 {
		s.writeError(w, protocol.NewError(protocol.CodeNotFound, "session not found"))
		return
	}
	if id == "" || id == sessionHash(cookie.Value) {
		s.clearCookie(w)
	}
	action := "web.session.revoke"
	if id == "" {
		action = "web.session.revoke_all"
	}
	s.logger.Audit(r.Context(), log.AuditEvent{Actor: actor.ID, Action: action, Result: "ok"})
	w.WriteHeader(http.StatusNoContent)
}
