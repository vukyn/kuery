// Package db is a dialect-aware connection factory for bun-backed services. It
// selects the SQLite or Postgres dialect/driver from a single Config and returns
// a ready *bun.DB. Both drivers are pure-Go (no CGO): SQLite via sqliteshim's
// modernc backend, Postgres via bun's first-party pgdriver. Callers attach their
// own query hooks after Open — none is attached here.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// Driver selects which database backend Open connects to.
type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
)

// defaultSQLitePragma matches the connection tuning used by the SQLite services.
// WAL + a busy timeout let separate processes (and concurrent writers) queue on
// the single SQLite writer lock instead of failing with SQLITE_BUSY;
// foreign_keys(1) enables ON DELETE CASCADE.
const defaultSQLitePragma = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

// Postgres pool defaults. These exist because a SERVERLESS Postgres (Neon and
// friends) suspends its compute after a few minutes of inactivity and drops the
// TCP connection without a FIN the client can see. database/sql keeps handing
// that dead connection out of the idle pool, pgdriver writes to it, and the
// caller gets a bare `EOF` (or, once the kernel gives up retransmitting, a
// `write: connection timed out`). It surfaces as a real failure, not a retry:
// pgdriver returns a plain net error rather than driver.ErrBadConn, so
// database/sql's built-in bad-connection retry never triggers.
//
// Retiring idle connections BEFORE the server can suspend under them is the fix,
// so the idle ceiling has to sit below the provider's suspend delay (Neon's is
// ~5 minutes). The lifetime cap is the backstop for the other direction — a
// connection killed mid-life by a pooler or a provider-side rotation.
const (
	defaultPostgresConnMaxIdleTime = 4 * time.Minute
	defaultPostgresConnMaxLifetime = 30 * time.Minute
)

// Config selects and configures the database connection. Driver defaults to
// sqlite when empty. For Postgres, PostgresDSN takes precedence; when it is empty
// the DSN is built from the discrete Host/Port/User/Password/DBName/SSLMode fields.
type Config struct {
	Driver Driver // "sqlite" | "postgres"; defaults to "sqlite" when empty

	// SQLite
	SQLitePath   string // e.g. "db/app.db"
	SQLitePragma string // optional; when empty the default pragma is used

	// Postgres
	PostgresDSN string // full DSN; when empty, built from the discrete fields below
	Host        string
	User        string
	Password    string
	DBName      string
	SSLMode     string
	Port        int

	// Pool limits, passed straight through to the *sql.DB. Each is three-state:
	// a POSITIVE value is used as given, ZERO takes the default for the driver
	// (for Postgres, the two constants above; otherwise database/sql's own), and
	// a NEGATIVE value means "no limit", which is how database/sql already reads
	// a non-positive duration. Pass a negative duration to opt a Postgres service
	// out of the idle/lifetime caps entirely.
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

// Open returns a *bun.DB for the configured driver. Unknown drivers return an
// error. No query hook is attached — callers add their own.
func Open(cfg Config) (*bun.DB, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = DriverSQLite
	}

	switch driver {
	case DriverSQLite:
		pragma := cfg.SQLitePragma
		if pragma == "" {
			pragma = defaultSQLitePragma
		}
		dsn := "file:" + cfg.SQLitePath + "?" + pragma
		sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database: %w", err)
		}
		applyPool(sqldb, cfg, DriverSQLite)
		return bun.NewDB(sqldb, sqlitedialect.New()), nil

	case DriverPostgres:
		dsn := cfg.PostgresDSN
		if dsn == "" {
			sslMode := cfg.SSLMode
			if sslMode == "" {
				sslMode = "disable"
			}
			dsn = fmt.Sprintf(
				"postgres://%s:%s@%s:%d/%s?sslmode=%s",
				cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, sslMode,
			)
		}
		sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		applyPool(sqldb, cfg, DriverPostgres)
		return bun.NewDB(sqldb, pgdialect.New()), nil

	default:
		return nil, fmt.Errorf("unknown database driver: %q", driver)
	}
}

// applyPool sets the *sql.DB pool limits from cfg, filling in the driver's
// defaults for any field left at zero. Only Postgres gets non-zero duration
// defaults: SQLite talks to a local file, so an idle connection there cannot go
// stale the way a suspended remote compute can, and retiring it would only pay
// for a reopen.
func applyPool(sqldb *sql.DB, cfg Config, driver Driver) {
	if cfg.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	idleTime := cfg.ConnMaxIdleTime
	lifetime := cfg.ConnMaxLifetime
	if driver == DriverPostgres {
		if idleTime == 0 {
			idleTime = defaultPostgresConnMaxIdleTime
		}
		if lifetime == 0 {
			lifetime = defaultPostgresConnMaxLifetime
		}
	}

	// A non-positive duration is database/sql's own encoding of "no limit", so a
	// caller's negative opt-out and an unset SQLite default both land here as a
	// no-op call with the same meaning.
	if idleTime != 0 {
		sqldb.SetConnMaxIdleTime(idleTime)
	}
	if lifetime != 0 {
		sqldb.SetConnMaxLifetime(lifetime)
	}
}
