package accounts_test

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/auth/accounts"
	"github.com/UruhaLushia/headdeck/internal/auth/policy"
	"github.com/UruhaLushia/headdeck/internal/config"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestPasskeyDisplayNames(t *testing.T) {
	ctx := context.Background()
	db, err := config.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := accounts.New(db.Database())
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.Save(ctx, accounts.Account{Label: "Alice", Role: policy.RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	id, err := hex.DecodeString("d548826e79b4db40a3d811116f7e8349")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, name, want string
		aaguid         []byte
	}{
		{"known", "Passkey", "Bitwarden", id},
		{"named", "My security key", "My security key", id},
		{"unknown", "Passkey", "Passkey", make([]byte, 16)},
	} {
		credential := webauthn.Credential{ID: []byte(item.id), Authenticator: webauthn.Authenticator{AAGUID: item.aaguid}}
		if err := store.SavePasskey(ctx, account.ID, accounts.Passkey{ID: item.id, Name: item.name, Credential: credential}, false); err != nil {
			t.Fatal(err)
		}
		if err := store.SavePasskey(ctx, account.ID, accounts.Passkey{ID: item.id, Credential: credential}, true); err != nil {
			t.Fatal(err)
		}
		keys, err := store.Passkeys(ctx, account.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if key.ID == item.id && key.Name != item.want {
				t.Fatalf("passkey %s name = %q, want %q", key.ID, key.Name, item.want)
			}
		}
	}
}

func TestOwnerAndHeadscaleUsername(t *testing.T) {
	ctx := context.Background()
	db, err := config.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := accounts.New(db.Database())
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.SaveWithPassword(ctx, accounts.Account{Label: "Owner", Role: policy.RoleOwner, HeadscaleUserID: "12", Username: "alice"}, "initial-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SyncHeadscaleUsers(ctx, map[string]string{"12": "alice@example.test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, "alice@example.test", "initial-password-123"); err != nil {
		t.Fatal("binding renamed the username but lost its password", err)
	}
	if _, err := store.Authenticate(ctx, "alice", "initial-password-123"); err == nil {
		t.Fatal("old username still logs in")
	}
	if err := store.SetPassword(ctx, a.ID, "other", "another-password-123"); err == nil {
		t.Fatal("bound username can be changed independently")
	}
	a.Role = policy.RoleAdmin
	if _, err := store.Save(ctx, *a); err == nil {
		t.Fatal("last owner can be demoted")
	}
	if _, err := store.Save(ctx, accounts.Account{Label: "Second", Role: policy.RoleOwner}); err != nil {
		t.Fatal(err)
	}
	a.Username = ""
	if _, err := store.Save(ctx, *a); err != nil {
		t.Fatal("owner transfer rejected", err)
	}
	before, err := store.Account(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SyncHeadscaleUsers(ctx, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	after, err := store.Account(ctx, a.ID)
	if err != nil || after.HeadscaleUserID != "" || after.Username != before.Username || after.SecurityVersion <= before.SecurityVersion {
		t.Fatalf("deleted Headscale binding = %+v, error = %v", after, err)
	}
	if _, err := store.Authenticate(ctx, before.Username, "initial-password-123"); err != nil {
		t.Fatal("deleting the Headscale user lost the local password", err)
	}
	if err := store.SyncHeadscaleUsers(ctx, map[string]string{"12": "replacement"}); err != nil {
		t.Fatal(err)
	}
	after, err = store.Account(ctx, a.ID)
	if err != nil || after.HeadscaleUserID != "" {
		t.Fatal("a recycled Headscale ID restored its old binding", err)
	}
}

func TestProvisionProviderUsername(t *testing.T) {
	ctx := context.Background()
	db, err := config.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := accounts.New(db.Database())
	if err != nil {
		t.Fatal(err)
	}
	identity := accounts.VerifiedIdentity{Identity: accounts.Identity{Issuer: "https://dex.example", Subject: "stable-subject"}, Name: "汐殇", Username: "xishang0128", VerifiedEmails: []string{"same@example.test"}}
	a, err := store.Provision(ctx, identity, policy.RolePending)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := store.Resolve(ctx, identity.Identity)
	if err != nil || resolved.ID != a.ID || resolved.Username != "xishang0128" || resolved.Label != "汐殇" || resolved.Role != policy.RolePending || resolved.HeadscaleUserID != "" {
		t.Fatalf("new account = %+v, error = %v", resolved, err)
	}
	if enabled, err := store.HasPassword(ctx, a.ID); err != nil || enabled {
		t.Fatal("provider signup enabled password login", err)
	}
	if again, err := store.Provision(ctx, identity, policy.RoleMember); err != nil || again.ID != a.ID || again.Role != policy.RolePending {
		t.Fatal("repeated login created another account", err)
	}
	identity.Issuer = "https://other.example"
	identity.VerifiedEmails = []string{"same@example.test"}
	b, err := store.Provision(ctx, identity, policy.RoleMember)
	if err != nil || b.ID == a.ID || b.Username == a.Username || b.Role != policy.RoleMember {
		t.Fatalf("cross-provider registration = %+v, error = %v", b, err)
	}
	identity.Subject = "another-subject"
	identity.Username = "昵称"
	if c, err := store.Provision(ctx, identity, policy.RoleMember); err != nil || c.Username == "" || c.ID == b.ID {
		t.Fatal("invalid provider username blocked registration", err)
	}
	if _, err := store.Provision(ctx, identity, policy.RoleOwner); err == nil {
		t.Fatal("registration can grant owner")
	}
	identity.Subject = "concurrent-signup"
	results := make(chan string, 8)
	for range 8 {
		go func() {
			a, err := store.Provision(ctx, identity, policy.RoleMember)
			if err != nil {
				t.Error(err)
				results <- ""
				return
			}
			results <- a.ID
		}()
	}
	id := <-results
	for range 7 {
		if other := <-results; other != id || id == "" {
			t.Error("concurrent signup created different accounts")
		}
	}
}

func TestIdentityBindingIsolationAndPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "settings.db")
	db, err := config.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := accounts.New(db.Database())
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.Save(ctx, accounts.Account{Label: "alice", Role: policy.RolePending, HeadscaleUserID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Save(ctx, accounts.Account{Label: "bob", Role: policy.RolePending})
	if err != nil {
		t.Fatal(err)
	}
	one := accounts.Identity{Issuer: "https://issuer-one.example", Subject: "same-subject"}
	two := accounts.Identity{Issuer: "https://issuer-two.example", Subject: "same-subject"}
	if err := store.Bind(ctx, a.ID, one); err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(ctx, b.ID, two); err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(ctx, b.ID, one); !errors.Is(err, accounts.ErrConflict) {
		t.Fatalf("rebind: %v", err)
	}
	if err := store.Bind(ctx, "missing", accounts.Identity{Issuer: one.Issuer, Subject: "unknown"}); err == nil {
		t.Fatal("foreign key was not enforced")
	}
	if err := store.Unbind(ctx, b.ID, one); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("cross-account unlink: %v", err)
	}
	if _, err := store.Resolve(ctx, accounts.Identity{Issuer: "unknown", Subject: one.Subject}); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("issuer isolation: %v", err)
	}
	a.Label = "renamed"
	a.Role = policy.RoleMember
	a.Disabled = true
	if _, err := store.Save(ctx, *a); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, one); !errors.Is(err, accounts.ErrDisabled) {
		t.Fatalf("disabled login: %v", err)
	}
	a.Disabled = false
	if _, err := store.Save(ctx, *a); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = config.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err = accounts.New(db.Database())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := store.Resolve(ctx, one)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != a.ID || resolved.HeadscaleUserID != "12" || resolved.Role != policy.RoleMember {
		t.Fatalf("binding changed after reopen: %+v", resolved)
	}
	b.HeadscaleUserID = "12"
	if _, err := store.Save(ctx, *b); !errors.Is(err, accounts.ErrConflict) {
		t.Fatalf("duplicate Headscale ownership: %v", err)
	}
}
