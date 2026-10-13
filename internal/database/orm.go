package database

import (
	"database/sql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(connection *sql.DB) (*gorm.DB, error) {
	return gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", Conn: connection}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
}
