package headscale

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureHandler serves a recorded response body for one path. The fixtures are
// captured shapes from real Headscale releases, so a client regression on an
// older or newer dialect shows up as a test failure rather than a runtime
// surprise.
func fixtureHandler(t *testing.T, path, fixture string) http.Handler {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
}

func TestFixturesDecodeAcrossVersions(t *testing.T) {
	cases := []struct {
		fixture    string
		wantNodes  int
		wantName   string
		wantOnline bool
		check      func(t *testing.T, n Node)
	}{
		{
			fixture: "nodes_0.22.json", wantNodes: 1, wantName: "old-node",
			check: func(t *testing.T, n Node) {
				if len(n.ForcedTags) != 1 || n.ForcedTags[0] != "tag:legacy" {
					t.Fatalf("forced tags = %+v", n.ForcedTags)
				}
				if len(n.InvalidTags) != 1 {
					t.Fatalf("invalid tags = %+v", n.InvalidTags)
				}
				if n.Tags != nil {
					t.Fatalf("0.22 has no tags field, got %+v", n.Tags)
				}
				if n.ApprovedRoutes != nil || n.SubnetRoutes != nil {
					t.Fatalf("0.22 has no route fields, got %+v/%+v", n.ApprovedRoutes, n.SubnetRoutes)
				}
				if n.Expiry == nil {
					t.Fatal("expiry should decode")
				}
			},
		},
		{
			fixture: "nodes_0.23.json", wantNodes: 1, wantName: "mid-node", wantOnline: true,
			check: func(t *testing.T, n Node) {
				if n.PreAuthKey == nil || !n.PreAuthKey.Reusable || !n.PreAuthKey.Ephemeral {
					t.Fatalf("preauth key = %+v", n.PreAuthKey)
				}
				if n.LastSeen != nil {
					t.Fatalf("null lastSeen must stay nil, got %v", n.LastSeen)
				}
				if len(n.IPAddresses) != 2 {
					t.Fatalf("ip addresses = %+v", n.IPAddresses)
				}
			},
		},
		{
			fixture: "nodes_0.26.json", wantNodes: 1, wantName: "route-node", wantOnline: true,
			check: func(t *testing.T, n Node) {
				if len(n.AvailableRoutes) != 2 || len(n.ApprovedRoutes) != 1 {
					t.Fatalf("routes = %+v / %+v", n.AvailableRoutes, n.ApprovedRoutes)
				}
				if n.SubnetRoutes == nil {
					t.Fatal("0.26 emits subnetRoutes as an empty list, not null")
				}
				if n.User == nil || n.User.Email != "carol@example.com" {
					t.Fatalf("user = %+v", n.User)
				}
			},
		},
		{
			fixture: "nodes_0.28.json", wantNodes: 1, wantName: "new-node", wantOnline: true,
			check: func(t *testing.T, n Node) {
				if len(n.Tags) != 2 {
					t.Fatalf("tags = %+v", n.Tags)
				}
				if n.ForcedTags != nil || n.ValidTags != nil {
					t.Fatalf("0.28 removed the old tag fields, got %+v", n.ForcedTags)
				}
				if n.User != nil {
					t.Fatalf("tagged nodes report a null user in 0.28+, got %+v", n.User)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			c := testClient(t, fixtureHandler(t, "/api/v1/node", tc.fixture))
			nodes, err := c.ListNodes(context.Background(), "")
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if len(nodes) != tc.wantNodes {
				t.Fatalf("got %d nodes", len(nodes))
			}
			n := nodes[0]
			if n.GivenName != tc.wantName {
				t.Fatalf("given name = %q, want %q", n.GivenName, tc.wantName)
			}
			if n.Online != tc.wantOnline {
				t.Fatalf("online = %v, want %v", n.Online, tc.wantOnline)
			}
			if tc.check != nil {
				tc.check(t, n)
			}
		})
	}
}

func TestFixturePreAuthKeysAndUsers(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/preauthkey":
			body, err := os.ReadFile(filepath.Join("testdata", "preauthkeys_0.29.json"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(body); err != nil {
				t.Errorf("write response: %v", err)
			}
		case "/api/v1/user":
			body, err := os.ReadFile(filepath.Join("testdata", "users_0.22.json"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(body); err != nil {
				t.Errorf("write response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	ctx := context.Background()

	keys, err := c.ListPreAuthKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("got %d keys", len(keys))
	}
	if keys[0].User == nil || keys[0].User.Name != "alice" {
		t.Fatalf("key user = %+v", keys[0].User)
	}
	if keys[0].Expiration == nil {
		t.Fatal("expiration should decode")
	}
	if keys[0].ACLTags == nil {
		t.Fatal("aclTags should decode as an empty list, not nil")
	}
	if !strings.Contains(keys[0].Key, "***") {
		t.Fatalf("0.29 masks the key prefix, got %q", keys[0].Key)
	}

	users, err := c.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "alice" {
		t.Fatalf("users = %+v", users)
	}
	// 0.22 users have no profile fields; they must stay empty rather than fail.
	if users[0].Email != "" || users[0].DisplayName != "" {
		t.Fatalf("0.22 user should have no profile fields, got %+v", users[0])
	}
}

func TestUserListMustBeAuthoritative(t *testing.T) {
	response := `{"users":[]}`
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(response)); err != nil {
			t.Error(err)
		}
	}))
	if users, err := c.ListUsers(context.Background()); err != nil || len(users) != 0 {
		t.Fatal("valid empty user list rejected", err)
	}
	for _, invalid := range []string{`{}`, `null`, `{"users":null}`, `{"users":[{}]}`, `{"users":[{"id":"1"},{"id":"1"}]}`} {
		response = invalid
		if _, err := c.ListUsers(context.Background()); err == nil {
			t.Fatal("invalid response accepted as an authoritative user list", invalid)
		}
	}
}
