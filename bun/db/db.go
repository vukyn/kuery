// Package db is a dialect-aware connection factory for bun-backed services. It
// selects the SQLite or Postgres dialect/driver from a single Config and returns
// a ready *bun.DB. Both drivers are pure-Go (no CGO): SQLite via sqliteshim's
// modernc backend, Postgres via bun's first-party pgdriver. Callers attach their
// own query hooks after Open — none is attached here.
package db

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
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
	SSLMode     string // when empty, defaults to "require" — set "disable" explicitly for a non-TLS server
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
			dsn = postgresDSN(cfg)
		}
		sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		applyPool(sqldb, cfg, DriverPostgres)
		return bun.NewDB(sqldb, pgdialect.New()), nil

	default:
		return nil, fmt.Errorf("unknown database driver: %q", driver)
	}
}

// defaultPostgresSSLMode is what an unset Config.SSLMode becomes.
//
// It is "require", not "disable". This library has no way to know whether the
// host it is handed is a loopback socket or a managed Postgres across the
// public internet, and a shared library that guesses wrong in the "disable"
// direction sends credentials and row data in cleartext without anyone being
// told. Guessing wrong in the "require" direction produces a connection error
// at startup naming sslmode, which is a question the operator can answer.
// Failing closed is the only default that cannot leak silently.
//
// This is a behaviour change for one caller shape only: a config that leaves
// SSLMode empty AND leaves PostgresDSN empty, pointing at a server without TLS.
// That caller must now set SSLMode: "disable" explicitly — which is the point,
// since cleartext then becomes a decision on the record instead of a default.
// It reaches no current consumer: every Postgres service passes SSLMode through
// from its own config, where it already carries an explicit default.
const defaultPostgresSSLMode = "require"

// postgresDSN assembles a Postgres URL from the discrete Config fields.
//
// It is built with net/url rather than fmt.Sprintf because the password is
// arbitrary user data being placed into URL syntax. A password containing "@"
// moves where the host ends, "/" moves where the path begins, "?" starts the
// query string and "#" truncates the URL at a fragment — so a sufficiently
// unlucky (or chosen) password could point the connection at a different host
// or replace the sslmode the caller asked for. url.UserPassword percent-encodes
// the userinfo and url.Values encodes the query, so every one of those
// characters survives as data.
func postgresDSN(cfg Config) string {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = defaultPostgresSSLMode
	}

	query := url.Values{}
	query.Set("sslmode", sslMode)

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:     "/" + cfg.DBName,
		RawQuery: query.Encode(),
	}
	return dsn.String()
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
