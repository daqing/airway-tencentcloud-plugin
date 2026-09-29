// Package migrations holds this module's schema as Go migrations. The host
// binary compiles them in through its blank import of the plugin module, and
// lib/migrate renders them per dialect, so the tables fit PostgreSQL, MySQL and
// SQLite alike — the SQL files these replace were written for PostgreSQL only.
//
// Nothing here is copied into a host project: `plugin:install` has no SQL
// migrations to install, and the host's own `go run . db:migrate` finds these
// definitions because they are part of its binary.
package migrations

import "github.com/daqing/airway/lib/migrate/schema"

// The versions match the SQL migrations these definitions replace, so a host
// that already applied them skips these instead of re-creating the tables.
func init() {
	schema.RegisterChange("20260929153501", "create_sms_verifications", func(m *schema.Migrator) {
		m.CreateTable("sms_verifications", func(t *schema.Table) {
			t.ID()
			t.String("phone", 20).Null(false)
			t.String("code", 6).Null(false)
			t.String("ip", 45).Null(false)
			t.DateTime("expires_at").Null(false)
			t.Boolean("consumed").Null(false).Default(false)
			t.Integer("attempts").Null(false).Default(0)
			t.Timestamps()

			// Both indexes cover the rate limit counts, which filter on the
			// phone or the IP plus created_at.
			t.Index("phone", "created_at").Name("index_sms_verifications_on_phone_created_at")
			t.Index("ip", "created_at").Name("index_sms_verifications_on_ip_created_at")
		})
	})

	schema.RegisterChange("20260929153502", "create_captchas", func(m *schema.Migrator) {
		m.CreateTable("captchas", func(t *schema.Table) {
			// The id is an ordinary autoincrement key, and `token` is the random
			// string captcha.Issue hands the client. Keeping them apart means the
			// client-facing identifier is never the row number, which is
			// enumerable, and the table gets a declared primary key.
			t.ID()
			t.String("token", 32).Null(false).Unique()
			t.String("answer", 8).Null(false)
			t.String("ip", 45).Null(false)
			t.DateTime("expires_at").Null(false)
			t.Boolean("consumed").Null(false).Default(false)
			t.Timestamps()

			t.Index("ip", "created_at").Name("index_captchas_on_ip_created_at")
		})
	})
}
