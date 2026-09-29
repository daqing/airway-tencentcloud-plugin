// Package testdb provides the in-memory SQLite database the tests and the dev
// server run against. It lives under install/ignore/, so it is never copied
// into a host project; a real host runs the migrations in
// install/host/db/migrate instead.
package testdb

import "github.com/daqing/airway/lib/repo"

// Schema is the SQLite spelling of install/host/db/migrate: no BIGSERIAL, no
// TIMESTAMPTZ, and DATETIME for timestamps, matching the framework's own test
// schema. Keep the two in step.
const Schema = `
CREATE TABLE sms_verifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  phone TEXT NOT NULL,
  code TEXT NOT NULL,
  ip TEXT NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed BOOLEAN NOT NULL DEFAULT FALSE,
  attempts INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX index_sms_verifications_on_phone_created_at ON sms_verifications (phone, created_at);
CREATE INDEX index_sms_verifications_on_ip_created_at ON sms_verifications (ip, created_at);

CREATE TABLE captchas (
  id TEXT PRIMARY KEY,
  answer TEXT NOT NULL,
  ip TEXT NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX index_captchas_on_ip_created_at ON captchas (ip, created_at);
`

// Setup opens a fresh in-memory SQLite database with Schema applied. The repo
// package caps this driver at one connection, so the :memory: database stays
// visible to every query.
func Setup() (*repo.DB, error) {
	db, err := repo.SetupDBWithDriver("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	if _, err := db.Conn().Exec(Schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
