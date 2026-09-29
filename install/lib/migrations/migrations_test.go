package migrations

import (
	"io"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/daqing/airway/lib/migrate"
	"github.com/daqing/airway/lib/repo"
)

// TestMigrationsApplyAndRollBack runs both directions against SQLite. The down
// migrations are derived from the up ones rather than written out, so nothing
// else would catch a db:rollback that left the tables behind.
func TestMigrationsApplyAndRollBack(t *testing.T) {
	dsn := "file:migrations-test?mode=memory&cache=shared"

	// The engine opens a connection per call and closes it again; this handle
	// keeps the named in-memory database alive across both.
	keep, err := repo.NewDB(dsn)
	if err != nil {
		t.Fatalf("open %s: %v", dsn, err)
	}
	t.Cleanup(func() { keep.Close() })

	opts := migrate.Options{DSN: dsn, Migrations: fstest.MapFS{}, Out: io.Discard}

	if err := migrate.Run(opts); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	for _, table := range []string{"sms_verifications", "captchas"} {
		if !hasTable(t, keep, table) {
			t.Fatalf("table %s is missing after Run", table)
		}
	}

	// A host runs db:migrate on every deploy, so a recorded version has to be
	// skipped rather than applied again — this is also the path a host that
	// already has these tables takes, given the matching versions.
	if err := migrate.Run(opts); err != nil {
		t.Fatalf("re-run migrations: %v", err)
	}
	for _, table := range []string{"sms_verifications", "captchas"} {
		if !hasTable(t, keep, table) {
			t.Fatalf("table %s is missing after a second Run", table)
		}
	}

	if err := migrate.Rollback(opts, 2); err != nil {
		t.Fatalf("roll back migrations: %v", err)
	}
	for _, table := range []string{"sms_verifications", "captchas"} {
		if hasTable(t, keep, table) {
			t.Fatalf("table %s still exists after Rollback", table)
		}
	}
}

func hasTable(t *testing.T, db *repo.DB, name string) bool {
	t.Helper()

	tables, err := repo.ListTables(db)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}

	return slices.Contains(tables, name)
}
