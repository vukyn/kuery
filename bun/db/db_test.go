package db

import (
	"database/sql"
	"reflect"
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
