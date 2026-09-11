package db

import (
	"database/sql"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// poolDuration reads one of *sql.DB's unexported pool durations. database/sql
// exposes no getter for SetConnMaxIdleTime / SetConnMaxLifetime and Stats()
// carries neither, so asserting that Open actually applied them means reading
// the field. The alternative — trusting the call site by eye — is what let the
// missing defaults ship in the first place.
//
// A rename in database/sql fails the test loudly rather than skipping it: a
// silently skipped assertion here is indistinguishable from the bug returning.
func poolDuration(t *testing.T, sqldb *sql.DB, field string) time.Duration {
	t.Helper()

	value := reflect.ValueOf(sqldb).Elem().FieldByName(field)
	if !value.IsValid() {
		t.Fatalf("database/sql internals moved: *sql.DB has no field %q — re-point this test at the new name", field)
	}
	return time.Duration(reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Int())
}

// TestOpenPostgresAppliesIdleAndLifetimeDefaults is the regression guard for the
// serverless-suspend failure: with no idle ceiling, a Neon-style compute
// suspends under an idle pooled connection and the next query returns a bare
// EOF that database/sql does not retry. The idle default must also stay BELOW
// the ~5min provider suspend delay, or it retires connections too late to help.
func TestOpenPostgresAppliesIdleAndLifetimeDefaults(t *testing.T) {
	database, err := Open(Config{
		Driver:      DriverPostgres,
		PostgresDSN: "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
	})
	if err != nil {
		t.Fatalf("Open postgres: %v", err)
	}
	defer database.Close()

	idleTime := poolDuration(t, database.DB, "maxIdleTime")
	if idleTime != defaultPostgresConnMaxIdleTime {
		t.Errorf("ConnMaxIdleTime = %v, want %v", idleTime, defaultPostgresConnMaxIdleTime)
	}
	if idleTime <= 0 || idleTime >= 5*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want a positive value under the ~5min serverless suspend delay", idleTime)
	}

	if lifetime := poolDuration(t, database.DB, "maxLifetime"); lifetime != defaultPostgresConnMaxLifetime {
		t.Errorf("ConnMaxLifetime = %v, want %v", lifetime, defaultPostgresConnMaxLifetime)
	}
}

// TestOpenPostgresHonoursExplicitPoolConfig covers the positive branch: a caller
// that sets the limits gets its own values, not the defaults.
func TestOpenPostgresHonoursExplicitPoolConfig(t *testing.T) {
	database, err := Open(Config{
		Driver:          DriverPostgres,
		PostgresDSN:     "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
		MaxOpenConns:    7,
		MaxIdleConns:    3,
		ConnMaxIdleTime: 90 * time.Second,
		ConnMaxLifetime: 11 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Open postgres: %v", err)
	}
	defer database.Close()

	if got := database.DB.Stats().MaxOpenConnections; got != 7 {
		t.Errorf("MaxOpenConnections = %d, want 7", got)
	}
	if got := poolDuration(t, database.DB, "maxIdleTime"); got != 90*time.Second {
		t.Errorf("ConnMaxIdleTime = %v, want 1m30s", got)
	}
	if got := poolDuration(t, database.DB, "maxLifetime"); got != 11*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 11m0s", got)
	}
}

// TestOpenPostgresNegativeDurationOptsOut covers the opt-out branch: a negative
// duration must reach the pool as "no limit" instead of being replaced by the
// default. Without the three-state handling this reads back as 4m.
func TestOpenPostgresNegativeDurationOptsOut(t *testing.T) {
	database, err := Open(Config{
		Driver:          DriverPostgres,
		PostgresDSN:     "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
		ConnMaxIdleTime: -1,
		ConnMaxLifetime: -1,
	})
	if err != nil {
		t.Fatalf("Open postgres: %v", err)
	}
	defer database.Close()

	if got := poolDuration(t, database.DB, "maxIdleTime"); got > 0 {
		t.Errorf("ConnMaxIdleTime = %v, want a non-positive (no-limit) value", got)
	}
	if got := poolDuration(t, database.DB, "maxLifetime"); got > 0 {
		t.Errorf("ConnMaxLifetime = %v, want a non-positive (no-limit) value", got)
	}
}

// TestOpenSQLiteLeavesDurationsUnset pins the driver split: the Postgres
// defaults exist for a suspending REMOTE compute, so applying them to a local
// SQLite file would only pay for reopens. SQLite keeps database/sql's defaults
// unless the caller asks otherwise.
func TestOpenSQLiteLeavesDurationsUnset(t *testing.T) {
	database, err := Open(Config{
		Driver:     DriverSQLite,
		SQLitePath: t.TempDir() + "/pool.db",
	})
	if err != nil {
		t.Fatalf("Open sqlite: %v", err)
	}
	defer database.Close()

	if got := poolDuration(t, database.DB, "maxIdleTime"); got != 0 {
		t.Errorf("sqlite ConnMaxIdleTime = %v, want 0 (unset)", got)
	}
	if got := poolDuration(t, database.DB, "maxLifetime"); got != 0 {
		t.Errorf("sqlite ConnMaxLifetime = %v, want 0 (unset)", got)
	}
}

// awkwardPassword contains every character that changes the MEANING of a URL:
// "@" ends the userinfo, "/" starts the path, "?" starts the query and "#"
// truncates at a fragment. A password is arbitrary user data, so all four are
// legal in one — and under fmt.Sprintf all four escape their field.
const awkwardPassword = "p/a?s#s@word"

// TestPostgresDSNDefaultsToRequireSSL pins the fail-closed default. This library
// cannot tell a loopback socket from a managed Postgres across the internet, so
// an unset SSLMode must not silently choose cleartext.
//
// Mutation: put the default back to "disable" and this fails.
func TestPostgresDSNDefaultsToRequireSSL(t *testing.T) {
	dsn := postgresDSN(Config{
		Host:   "db.example.com",
		Port:   5432,
		User:   "app",
		DBName: "appdb",
	})

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("built an unparseable DSN %q: %v", dsn, err)
	}
	if got := parsed.Query().Get("sslmode"); got != "require" {
		t.Fatalf("sslmode = %q, want %q — an unset SSLMode must not fall back to cleartext", got, "require")
	}
}

// TestPostgresDSNHonoursExplicitSSLMode covers the other side of the default: a
// caller that genuinely wants cleartext (a local dev Postgres) can still say so,
// and saying so is now a decision on the record rather than an accident.
func TestPostgresDSNHonoursExplicitSSLMode(t *testing.T) {
	dsn := postgresDSN(Config{
		Host:    "localhost",
		Port:    5432,
		User:    "app",
		DBName:  "appdb",
		SSLMode: "disable",
	})

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("built an unparseable DSN %q: %v", dsn, err)
	}
	if got := parsed.Query().Get("sslmode"); got != "disable" {
		t.Fatalf("sslmode = %q, want %q", got, "disable")
	}
}

// TestPostgresDSNEscapesCredentials is the regression guard for the injection:
// a password carrying URL metacharacters must survive as DATA. Under the old
// fmt.Sprintf the "@" alone redirected the connection to a different host, and
// the "?" let the password append its own sslmode — a wrong password could
// therefore downgrade the connection to cleartext or point it at an attacker's
// server.
//
// Every field is checked, not just the password, because the failure mode is
// that one field's contents get read as another field.
//
// Mutation: rebuild the DSN with fmt.Sprintf and this fails.
func TestPostgresDSNEscapesCredentials(t *testing.T) {
	dsn := postgresDSN(Config{
		Host:     "db.example.com",
		Port:     5432,
		User:     "app@corp",
		Password: awkwardPassword,
		DBName:   "appdb",
		SSLMode:  "verify-full",
	})

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("built an unparseable DSN %q: %v", dsn, err)
	}

	if parsed.Scheme != "postgres" {
		t.Errorf("scheme = %q, want postgres", parsed.Scheme)
	}
	if parsed.Host != "db.example.com:5432" {
		t.Errorf("host = %q, want db.example.com:5432 — the password escaped its field and moved the host", parsed.Host)
	}
	if got := parsed.User.Username(); got != "app@corp" {
		t.Errorf("user = %q, want app@corp", got)
	}
	password, set := parsed.User.Password()
	if !set || password != awkwardPassword {
		t.Errorf("password = %q (set=%v), want %q", password, set, awkwardPassword)
	}
	if parsed.Path != "/appdb" {
		t.Errorf("path = %q, want /appdb — the password's %q escaped into the path", parsed.Path, "/")
	}
	if got := parsed.Query().Get("sslmode"); got != "verify-full" {
		t.Errorf("sslmode = %q, want verify-full — the password's %q overrode the caller's choice", got, "?")
	}
	// The password must not appear raw anywhere in the DSN: if it does, some
	// part of it was never encoded and is still being read as syntax.
	if strings.Contains(dsn, awkwardPassword) {
		t.Errorf("the password appears unencoded in the DSN: %q", dsn)
	}
}

// TestOpenPostgresPrefersExplicitDSN records why the sslmode default change
// reaches no current consumer: every Postgres service passes a full
// PostgresDSN, which wins over the discrete fields entirely.
func TestOpenPostgresPrefersExplicitDSN(t *testing.T) {
	database, err := Open(Config{
		Driver:      DriverPostgres,
		PostgresDSN: "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
		Host:        "ignored.example.com",
		SSLMode:     "",
	})
	if err != nil {
		t.Fatalf("Open postgres: %v", err)
	}
	defer database.Close()
}
