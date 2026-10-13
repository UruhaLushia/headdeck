package daemon_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/UruhaLushia/headdeck/internal/auth/principal"
	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

func TestUserManagementAndAccountBindingPermissions(t *testing.T) {
	var mu sync.Mutex
	users := []map[string]any{{"id": "13", "name": "other"}, {"id": "14", "name": "oidc-user", "provider": "oidc"}}
	owner := "12"
	renames := 0
	f := start(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body any = map[string]any{}
		switch {
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodGet:
			body = map[string]any{"users": users}
		case r.URL.Path == "/api/v1/user" && r.Method == http.MethodPost:
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			if req["displayName"] != "Display" || req["email"] != "a@example.test" {
				t.Error("creation profile did not reach Headscale")
			}
			req["id"] = "12"
			users = append(users, req)
			body = map[string]any{"user": req}
		case strings.HasPrefix(r.URL.Path, "/api/v1/user/12/rename/"):
			renames++
			users[len(users)-1]["name"] = strings.TrimPrefix(r.URL.Path, "/api/v1/user/12/rename/")
			body = map[string]any{"user": users[len(users)-1]}
		case r.URL.Path == "/api/v1/user/12" && r.Method == http.MethodDelete:
			users = users[:len(users)-1]
		case r.URL.Path == "/api/v1/node":
			body = map[string]any{"nodes": []any{map[string]any{"id": "8", "user": map[string]any{"id": owner, "name": "owner"}}}}
		case r.URL.Path == "/api/v1/node/8/user":
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			owner = req["user"]
			body = map[string]any{"node": map[string]any{"id": "8", "user": map[string]any{"id": owner}}}
		case r.URL.Path == "/version":
			body = map[string]any{"version": "0.26.1"}
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	denied := func(err error, code protocol.ErrorCode) {
		t.Helper()
		var e *client.StatusError
		if !errors.As(err, &e) || e.Proto.Code != code {
			t.Fatalf("error = %v, want %s", err, code)
		}
	}
	f.authorize(t, principal.RolePending)
	_, err := f.client.CreateUser(ctx, protocol.CreateUserRequest{Name: "denied"})
	denied(err, protocol.CodePermissionDenied)
	_, err = f.client.SaveAccount(ctx, "", protocol.Account{Label: "denied", Role: principal.RoleAdmin})
	denied(err, protocol.CodePermissionDenied)
	f.authorize(t, principal.RoleAdmin)
	_, err = f.client.SaveAccount(ctx, "", protocol.Account{Label: "denied", Role: principal.RoleOwner})
	denied(err, protocol.CodePermissionDenied)
	f.authorize(t, principal.RoleOwner)
	u, err := f.client.CreateUser(ctx, protocol.CreateUserRequest{Name: "alice", DisplayName: "Display", Email: "a@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.client.SaveAccount(ctx, "", protocol.Account{Label: "alice", Role: principal.RolePending, HeadscaleUserID: u.ID})
	if err != nil {
		t.Fatal(err)
	}
	i := protocol.Identity{Issuer: "https://issuer.example", Subject: "alice-stable-id"}
	if err := f.client.BindIdentity(ctx, a.ID, i, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.client.SaveAccount(ctx, "", protocol.Account{Label: "Other", Role: principal.RolePending, HeadscaleUserID: "13"}); err != nil {
		t.Fatal(err)
	}
	_, err = f.client.RenameUser(ctx, u.ID, "other")
	denied(err, protocol.CodeConflict)
	mu.Lock()
	changed := renames != 0
	mu.Unlock()
	if changed {
		t.Fatal("username conflict reached Headscale before validation")
	}
	u, err = f.client.RenameUser(ctx, u.ID, "alice-renamed")
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := f.client.IdentityBindings(ctx, a.ID)
	if err != nil || len(bindings.Bindings) != 1 {
		t.Fatalf("rename changed binding: %v %+v", err, bindings)
	}
	denied(f.client.DeleteUser(ctx, u.ID, u.Name), protocol.CodeBadRequest)
	a.HeadscaleUserID = ""
	if _, err := f.client.SaveAccount(ctx, a.ID, *a); err != nil {
		t.Fatal(err)
	}
	denied(f.client.DeleteUser(ctx, u.ID, u.Name), protocol.CodeBadRequest)
	if _, err := f.client.MoveNode(ctx, "8", "13"); err != nil {
		t.Fatal(err)
	}
	denied(f.client.DeleteUser(ctx, u.ID, "wrong-name"), protocol.CodeBadRequest)
	if err := f.client.DeleteUser(ctx, u.ID, u.Name); err != nil {
		t.Fatal(err)
	}
	_, err = f.client.RenameUser(ctx, "14", "renamed-oidc")
	denied(err, protocol.CodeBadRequest)
	f.authorize(t, principal.RoleMember)
	denied(f.client.BindIdentity(ctx, a.ID, i, true), protocol.CodePermissionDenied)
}

func TestResourceDiffStream(t *testing.T) {
	var mu sync.Mutex
	nodes := []map[string]any{{"id": "1", "givenName": "keep"}, {"id": "2", "givenName": "old"}, {"id": "3", "givenName": "remove"}}
	users := []map[string]any{{"id": "12", "name": "old-user"}}
	failNodes := false
	f := start(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body any = map[string]any{}
		switch r.URL.Path {
		case "/api/v1/node":
			if failNodes {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			body = map[string]any{"nodes": nodes}
		case "/api/v1/user":
			body = map[string]any{"users": users}
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Error(err)
		}
	})
	f.authorize(t, principal.RoleAdmin)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	baseline, err := f.client.Nodes(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	userBaseline, err := f.client.Users(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 1)
	changes := make(chan protocol.Event, 8)
	done := make(chan error, 1)
	go func() {
		done <- f.client.WatchEvents(ctx, client.WatchOptions{
			OnCursor: func(uint64) {
				select {
				case ready <- struct{}{}:
				default:
				}
			},
			OnEvent: func(ev protocol.Event) error { changes <- ev; return nil },
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("watch did not stop")
		}
	})
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("watch did not open")
	}
	mu.Lock()
	nodes = []map[string]any{nodes[0], {"id": "2", "givenName": "updated"}, {"id": "4", "givenName": "new"}}
	users = []map[string]any{{"id": "12", "name": "updated-user"}}
	mu.Unlock()
	seenNodes, seenUsers := false, false
	for !seenNodes || !seenUsers {
		select {
		case ev := <-changes:
			switch ev.Type {
			case protocol.EventNodesChanged:
				var diff protocol.CollectionDiff[protocol.Node]
				if err := json.Unmarshal(ev.Data, &diff); err != nil {
					t.Fatal(err)
				}
				if diff.BaseRevision != baseline.Revision || diff.Revision != baseline.Revision+1 || len(diff.Upsert) != 2 || len(diff.Removed) != 1 || diff.Removed[0] != "3" {
					t.Fatalf("unexpected node diff: %+v", diff)
				}
				for _, n := range diff.Upsert {
					if n.ID == "1" {
						t.Fatal("unchanged node was pushed")
					}
				}
				seenNodes = true
			case protocol.EventUsersChanged:
				var diff protocol.CollectionDiff[protocol.User]
				if err := json.Unmarshal(ev.Data, &diff); err != nil {
					t.Fatal(err)
				}
				if diff.BaseRevision != userBaseline.Revision || len(diff.Upsert) != 1 || diff.Upsert[0].Name != "updated-user" {
					t.Fatalf("unexpected user diff: %+v", diff)
				}
				seenUsers = true
			}
		case <-ctx.Done():
			t.Fatal("external changes were not pushed")
		}
	}
	if current, err := f.client.Nodes(ctx, ""); err != nil || current.Revision != baseline.Revision+1 {
		t.Fatal("unchanged fetch changed the revision", current, err)
	}
	mu.Lock()
	failNodes = true
	mu.Unlock()
	if _, err := f.client.Nodes(ctx, ""); err == nil {
		t.Fatal("failed upstream fetch succeeded")
	}
	select {
	case ev := <-changes:
		t.Fatalf("unchanged or failed fetch emitted an event: %+v", ev)
	default:
	}
}
