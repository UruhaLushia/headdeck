package identity

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"

	"github.com/UruhaLushia/headdeck/internal/database"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type Binding struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Before   string `json:"before"`
	Provider string `json:"provider"`
	After    string `json:"after"`
}

type userRecord struct {
	ID                 uint64 `gorm:"primaryKey"`
	Name               string
	ProviderIdentifier sql.NullString `gorm:"uniqueIndex"`
	Provider           string
	DeletedAt          gorm.DeletedAt
}

func (userRecord) TableName() string { return "users" }

type schemaObject struct{ Type, Name, TblName string }

func (schemaObject) TableName() string { return "sqlite_master" }

func updateUsers(ctx context.Context, path string, bindings []Binding, restore bool) error {
	connection, err := sql.Open("sqlite", "file:"+path+"?mode=rw&_pragma=journal_mode(DELETE)&_pragma=foreign_keys(ON)")
	if err != nil {
		return err
	}
	defer connection.Close()
	connection.SetMaxOpenConns(1)
	db, err := database.Open(connection)
	if err != nil {
		return err
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var schema []schemaObject
		if err := tx.Where(&schemaObject{TblName: "users"}).Find(&schema).Error; err != nil {
			return err
		}
		table := false
		for _, object := range schema {
			if object.Type == "trigger" || object.Type == "view" {
				return fmt.Errorf("unsupported Headscale users schema")
			}
			table = table || object.Type == "table"
		}
		if !table {
			return fmt.Errorf("headscale users table is missing")
		}
		var users []userRecord
		if err := tx.Find(&users).Error; err != nil {
			return err
		}
		wanted := make(map[uint64]Binding, len(bindings))
		for _, b := range bindings {
			if b.ID == 0 || b.After == "" || wanted[b.ID].ID != 0 {
				return fmt.Errorf("invalid Headscale identity bindings")
			}
			wanted[b.ID] = b
		}
		found := 0
		var updates []userRecord
		for _, user := range users {
			b, bound := wanted[user.ID]
			if !bound {
				if !restore && (user.Provider == "oidc" || user.ProviderIdentifier.String != "") {
					return fmt.Errorf("headscale user %d must be explicitly bound before takeover", user.ID)
				}
				continue
			}
			found++
			from, to, fromProvider, toProvider := b.Before, b.After, b.Provider, "oidc"
			if restore {
				from, to, fromProvider, toProvider = to, from, toProvider, fromProvider
			}
			if user.Name != b.Name {
				return fmt.Errorf("headscale user %d changed during takeover", user.ID)
			}
			if user.ProviderIdentifier.String == to && user.Provider == toProvider {
				continue
			}
			if user.ProviderIdentifier.String != from || user.Provider != fromProvider {
				return fmt.Errorf("headscale identity %d changed during takeover", user.ID)
			}
			user.ProviderIdentifier = sql.NullString{String: to, Valid: to != ""}
			user.Provider = toProvider
			updates = append(updates, user)
		}
		if found != len(bindings) {
			return fmt.Errorf("a bound Headscale user is missing")
		}
		// Reserve distinct identifiers within the transaction so account bindings can be swapped.
		for _, user := range updates {
			if err := tx.Model(&user).UpdateColumn("provider_identifier", "headdeck:pending:"+rand.Text()).Error; err != nil {
				return err
			}
		}
		for _, user := range updates {
			if err := tx.Model(&user).UpdateColumns(map[string]any{"provider_identifier": user.ProviderIdentifier, "provider": user.Provider}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return connection.Close()
}
