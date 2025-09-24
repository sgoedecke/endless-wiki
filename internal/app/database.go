package app

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// NewDB opens a MySQL connection using sensible defaults.
func NewDB(cfg Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	return db, nil
}
