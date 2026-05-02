package store

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetDialect("sqlite3")

	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}

	return nil
}
