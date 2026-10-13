package web

import (
	"net/http"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/policyedit"
)

type policyObjectUpdate struct {
	Revision string              `json:"revision"`
	Item     policyedit.Item     `json:"item"`
	Values   map[string][]string `json:"values"`
	Remove   bool                `json:"remove"`
}

func (s *Server) managedRoutes(mux *http.ServeMux) {
	read := []principal.Permission{principal.PermSettingsRead}
	write := []principal.Permission{principal.PermSettingsWrite}
	apply := []principal.Permission{principal.PermSettingsWrite, principal.PermServiceManage}
	s.endpoint(mux, "GET /api/headscale/config", read, func(r *http.Request) (any, error) { return s.svc.ManagedConfig(r.Context()) })
	s.endpoint(mux, "PATCH /api/headscale/config", write, withBody(func(r *http.Request, v protocol.ManagedConfigUpdate) (any, error) {
		return s.svc.SaveManagedConfig(r.Context(), v)
	}))
	s.endpoint(mux, "DELETE /api/headscale/config", write, func(r *http.Request) (any, error) {
		return map[string]bool{"discarded": true}, s.svc.DiscardManagedConfig(r.Context())
	})
	s.endpoint(mux, "POST /api/headscale/config/apply", apply, withBody(func(r *http.Request, v protocol.DNSApply) (any, error) {
		return s.svc.ApplyManagedConfig(r.Context(), v.Revision, actor(r))
	}))
	s.endpoint(mux, "GET /api/dns", []principal.Permission{principal.PermDNSRead}, func(r *http.Request) (any, error) { return s.svc.DNS(r.Context()) })
	s.endpoint(mux, "PUT /api/dns", []principal.Permission{principal.PermDNSWrite}, withBody(func(r *http.Request, v protocol.DNSUpdate) (any, error) { return s.svc.SaveDNS(r.Context(), v) }))
	s.endpoint(mux, "DELETE /api/dns", []principal.Permission{principal.PermDNSWrite}, func(r *http.Request) (any, error) {
		return map[string]bool{"discarded": true}, s.svc.DiscardDNS(r.Context())
	})
	s.endpoint(mux, "POST /api/dns/apply", []principal.Permission{principal.PermDNSWrite, principal.PermServiceManage}, withBody(func(r *http.Request, v protocol.DNSApply) (any, error) {
		return s.svc.ApplyDNS(r.Context(), v.Revision, actor(r))
	}))
	s.endpoint(mux, "GET /api/policy", []principal.Permission{principal.PermPolicyRead}, func(r *http.Request) (any, error) { return s.svc.Policy(r.Context()) })
	s.endpoint(mux, "GET /api/policy/objects", []principal.Permission{principal.PermPolicyRead}, s.policyObjects)
	s.endpoint(mux, "POST /api/policy/objects", []principal.Permission{principal.PermPolicyWrite}, withBody(func(r *http.Request, v policyObjectUpdate) (any, error) {
		return s.svc.EditPolicy(r.Context(), v.Revision, v.Item, v.Values, v.Remove)
	}))
	s.endpoint(mux, "DELETE /api/policy", []principal.Permission{principal.PermPolicyWrite}, func(r *http.Request) (any, error) {
		return map[string]bool{"discarded": true}, s.svc.DiscardPolicy(r.Context())
	})
	s.endpoint(mux, "POST /api/policy/apply", []principal.Permission{principal.PermPolicyWrite, principal.PermServiceManage}, withBody(func(r *http.Request, v protocol.DNSApply) (any, error) {
		return s.svc.ApplyPolicy(r.Context(), v.Revision, actor(r))
	}))
	s.endpoint(mux, "POST /api/headscale/{action}", []principal.Permission{principal.PermServiceManage}, func(r *http.Request) (any, error) {
		return s.svc.StartHeadscaleAction(r.Context(), protocol.ServiceAction(r.PathValue("action")), actor(r))
	})
	s.endpoint(mux, "GET /api/operations/{id}", []principal.Permission{principal.PermOperationsRead}, func(r *http.Request) (any, error) {
		out, ok := s.svc.Operations().Get(r.PathValue("id"))
		if !ok {
			return nil, protocol.NewError(protocol.CodeNotFound, "operation not found")
		}
		return out, nil
	})
}

func (s *Server) policyObjects(r *http.Request) (any, error) {
	out, err := s.svc.Policy(r.Context())
	if err != nil {
		return nil, err
	}
	document, err := policyedit.Parse(out.Policy)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeBadRequest, "cannot read structured policy: %v", err)
	}
	type entry struct {
		Item   policyedit.Item    `json:"item"`
		Fields []policyedit.Field `json:"fields"`
	}
	items := []entry{}
	for _, item := range document.Items() {
		items = append(items, entry{Item: item, Fields: document.Fields(item)})
	}
	return struct {
		Settings *protocol.PolicySettings `json:"settings"`
		Items    []entry                  `json:"items"`
	}{out, items}, nil
}
