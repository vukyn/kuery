// Package db is a dialect-aware connection factory for bun-backed services. It
// selects the SQLite or Postgres dialect/driver from a single Config and returns
// a ready *bun.DB. Both drivers are pure-Go (no CGO): SQLite via sqliteshim's
// modernc backend, Postgres via bun's first-party pgdriver. Callers attach their
// own query hooks after Open — none is attached here.
package db

import (
	"database/sql"
	"fmt"

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
		return bun.NewDB(sqldb, pgdialect.New()), nil

	default:
		return nil, fmt.Errorf("unknown database driver: %q", driver)
	}
}
