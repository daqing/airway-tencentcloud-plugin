// Package testdb provides the in-memory SQLite database the tests and the dev
// server run against. It lives under install/ignore/, so it is never copied
// into a host project; a real host runs the same migrations itself.
package testdb

import (
	"fmt"
	"io"
	"sync/atomic"
	"testing/fstest"

	"github.com/daqing/airway/lib/migrate"
	"github.com/daqing/airway/lib/repo"

	// Registers the module's Go migrations with lib/migrate.
	_ "github.com/daqing/airway-tencentcloud-plugin/install/lib/migrations"
)

var databases atomic.Uint64

// Setup opens a fresh in-memory SQLite database with the module's migrations
// applied.
//
// The database is named and shared-cache because the migrate engine opens a
// connection of its own: with a plain :memory: DSN it would migrate a second,
// empty database and every query would then fail on a missing table. The name
// is unique per call, so each test starts from an empty schema.
func Setup() (*repo.DB, error) {
	return SetupOn(fmt.Sprintf("file:testdb-%d?mode=memory&cache=shared", databases.Add(1)))
}

// SetupOn opens the database the DSN points at — PostgreSQL, MySQL or SQLite —
// and applies the module's migrations to it. The dev server uses this to run
// against a real server, which is how the migrations get exercised on a dialect
// other than SQLite.
func SetupOn(dsn string) (*repo.DB, error) {
	db, err := repo.SetupDB(dsn)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(db, dsn); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func runMigrations(db *repo.DB, dsn string) error {
	// The registered DSL migrations are the whole schema; there are no SQL
	// files to walk, but the engine requires a filesystem either way.
	if err := migrate.Run(migrate.Options{
		DSN:        dsn,
		Migrations: fstest.MapFS{},
		Out:        io.Discard,
	}); err != nil {
		return err
	}

	// migrate.Run reports success on an empty registry too, which would leave
	// every test failing on a missing table with no hint as to why, so probe
	// what the queries need.
	for _, table := range []string{"sms_verifications", "captchas"} {
		if _, err := db.Conn().Exec("SELECT 1 FROM " + table + " LIMIT 1"); err != nil {
			return fmt.Errorf("table %s: %w", table, err)
		}
	}

	return nil
}
