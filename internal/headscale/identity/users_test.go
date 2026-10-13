package identity

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/UruhaLushia/headdeck/internal/database"
	"gorm.io/gorm"
)

type nodeRecord struct {
	ID, UserID uint64
	Name       string
}

func (nodeRecord) TableName() string { return "nodes" }

func openFixture(t *testing.T, path string) (*gorm.DB, *sql.DB) {
	t.Helper()
	c, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	return db, c
}

func TestIdentityTakeoverAndRollback(t *testing.T) {
	base := t.TempDir()
	dir, work := filepath.Join(base, "headscale"), filepath.Join(base, "headdeck")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, databaseName)
	db, connection := openFixture(t, path)
	if err := db.AutoMigrate(&userRecord{}, &nodeRecord{}); err != nil {
		t.Fatal(err)
	}
	u := userRecord{ID: 10, Name: "github-login", Provider: "oidc", ProviderIdentifier: sql.NullString{String: "https://dex.example/old-subject", Valid: true}}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	second := userRecord{ID: 20, Name: "second-user", Provider: "oidc", ProviderIdentifier: sql.NullString{String: "https://dex.example/second", Valid: true}}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&nodeRecord{ID: 1, UserID: 10, Name: "device"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	binding := Binding{ID: 10, Name: u.Name, Before: u.ProviderIdentifier.String, Provider: u.Provider, After: "https://headdeck.example/oidc/account-id"}
	bindings := []Binding{binding, {ID: 20, Name: second.Name, Before: second.ProviderIdentifier.String, Provider: second.Provider, After: "https://headdeck.example/oidc/second-account"}}
	ctx := context.Background()
	changed, err := applyIn(ctx, dir, work, uint32(os.Geteuid()), bindings, false)
	if err != nil || !changed {
		t.Fatalf("apply: changed=%v error=%v", changed, err)
	}
	swap := []Binding{
		{ID: 10, Name: u.Name, Before: bindings[0].After, Provider: "oidc", After: bindings[1].After},
		{ID: 20, Name: second.Name, Before: bindings[1].After, Provider: "oidc", After: bindings[0].After},
	}
	if _, err := applyIn(ctx, dir, work, uint32(os.Geteuid()), swap, false); err != nil {
		t.Fatal("account swap failed", err)
	}
	if _, err := applyIn(ctx, dir, work, uint32(os.Geteuid()), swap, true); err != nil {
		t.Fatal("account swap rollback failed", err)
	}
	db, connection = openFixture(t, path)
	var current userRecord
	if err := db.First(&current, 10).Error; err != nil {
		t.Fatal(err)
	}
	if current.ID != 10 || current.Name != u.Name || current.ProviderIdentifier.String != binding.After {
		t.Fatalf("wrong identity: %+v", current)
	}
	if err := db.Create(&nodeRecord{ID: 2, UserID: 10, Name: "registered-after-switch"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	changed, err = applyIn(ctx, dir, work, uint32(os.Geteuid()), bindings, true)
	if err != nil || !changed {
		t.Fatalf("restore: changed=%v error=%v", changed, err)
	}
	db, connection = openFixture(t, path)
	defer connection.Close()
	if err := db.First(&current, 10).Error; err != nil {
		t.Fatal(err)
	}
	if current.ProviderIdentifier != u.ProviderIdentifier || current.Provider != u.Provider {
		t.Fatalf("identity was not restored: %+v", current)
	}
	var nodes []nodeRecord
	if err := db.Find(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].UserID != 10 || nodes[1].UserID != 10 {
		t.Fatalf("device data changed: %+v", nodes)
	}
	if _, err := os.Stat(filepath.Join(work, "identity.sqlite")); !os.IsNotExist(err) {
		t.Fatal("temporary database was retained")
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	bad := binding
	bad.Before = "wrong-subject"
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := applyIn(ctx, dir, work, uint32(os.Geteuid()), []Binding{bad}, false); err == nil || changed {
		t.Fatal("mismatched binding was written")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed takeover modified the database")
	}
	if err := os.WriteFile(path+"-wal", []byte("uncheckpointed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := applyIn(ctx, dir, work, uint32(os.Geteuid()), []Binding{binding}, false); err == nil || changed {
		t.Fatal("live WAL was accepted")
	}
	if err := os.Remove(path + "-wal"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path+".original", path); err != nil {
		t.Fatal(err)
	}
	if changed, err := applyIn(ctx, dir, work, uint32(os.Geteuid()), []Binding{binding}, false); err == nil || changed {
		t.Fatal("symlink database was accepted")
	}
}

func TestIdentityTakeoverWithoutBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, connection := openFixture(t, path)
	if err := db.AutoMigrate(&userRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := updateUsers(ctx, path, nil, false); err != nil {
		t.Fatal("empty Headscale database requires an owner binding", err)
	}
	db, connection = openFixture(t, path)
	if err := db.Create(&userRecord{Name: "unbound", Provider: "oidc", ProviderIdentifier: sql.NullString{String: "https://old.example/subject", Valid: true}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := updateUsers(ctx, path, nil, false); err == nil {
		t.Fatal("unbound existing OIDC identity was accepted")
	}
}
