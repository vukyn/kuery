// Package migrate is a dialect-agnostic database migration runner for services
// that track schema changes with pure-Go migrations recorded in a "migrations"
// bookkeeping table. It operates on a *bun.DB only — opening the connection and
// selecting the dialect/driver stays in each service's CLI shim.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

// ErrNoMigrationsToRollback is returned by RollbackLast when there are no
// executed migrations left to roll back. Detect it with errors.Is.
var ErrNoMigrationsToRollback = errors.New("no migrations to rollback")

// Migration represents a single database migration with its up/down steps.
//
// Up and Down receive a bun.IDB — the interface satisfied by both *bun.DB and
// bun.Tx — so the runner can thread its per-migration transaction handle into
// the steps. Each migration runs inside a transaction: the schema change and
// its bookkeeping row commit or roll back together atomically.
type Migration struct {
	Name string
	Up   func(bun.IDB) error
	Down func(bun.IDB) error
}

// Stats holds statistics about a migration run.
type Stats struct {
	TotalSkipped int
	TotalSuccess int
}

// History is the bun model for the "migrations" bookkeeping table.
type History struct {
	bun.BaseModel `bun:"table:migrations,alias:mig"`
	ID            int64     `bun:"id,pk,notnull"`
	Name          string    `bun:"name,notnull"`
	ExecutedAt    time.Time `bun:"executed_at,notnull"`
}

// Run executes all pending migrations against the given database. Each pending
// migration runs inside its own transaction: the migration's Up step and the
// bookkeeping row that records it commit together, or roll back together on
// failure — so a failed migration never leaves partial schema state.
func Run(db *bun.DB, migrations []Migration) (Stats, error) {
	stats := Stats{}

	// Create migrations table if it doesn't exist. The bookkeeping DDL is
	// dialect-aware: the runner supplies both `id` (lastMigratedID+1) and
	// `executed_at` (time.Now()) explicitly in its History inserts, so neither
	// column needs server-side auto-generation — Postgres uses a plain BIGINT PK
	// rather than serial/identity. The SQLite branch is kept byte-for-byte
	// identical to the original DDL to pose zero risk to existing databases.
	var createTableSQL string
	switch db.Dialect().Name() {
	case dialect.PG:
		createTableSQL = `
		CREATE TABLE IF NOT EXISTS migrations (
			id BIGINT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			executed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`
	case dialect.SQLite:
		createTableSQL = `
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`
	default:
		return stats, fmt.Errorf("unsupported dialect for migrations table: %s", db.Dialect().Name())
	}
	if _, err := db.Exec(createTableSQL); err != nil {
		return stats, fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get executed migrations
	var executedMigrations []History
	ctx := context.Background()
	if err := db.NewSelect().Table("migrations").Scan(ctx, &executedMigrations); err != nil {
		return stats, fmt.Errorf("failed to get executed migrations: %w", err)
	}

	// Execute pending migrations. The next id is max(existing id) + 1 — computed
	// over all rows rather than the last scanned one, because the SELECT above has
	// no ORDER BY and Postgres does not guarantee row order, so the last element is
	// not necessarily the highest id. Getting this wrong collides on the migrations
	// primary key (e.g. the first incremental migration run after a baseline that
	// stamped ids 1..N).
	lastMigratedID := int64(0)
	for _, executed := range executedMigrations {
		if executed.ID > lastMigratedID {
			lastMigratedID = executed.ID
		}
	}
	for _, migration := range migrations {
		// Check if migration already executed
		executed := slices.ContainsFunc(executedMigrations, func(m History) bool {
			return m.Name == migration.Name
		})

		if !executed {
			log.Printf("Running migration: %s", migration.Name)

			// Start transaction
			tx, err := db.Begin()
			if err != nil {
				return stats, fmt.Errorf("failed to begin transaction for migration %s: %w", migration.Name, err)
			}

			// Execute migration inside the transaction
			if err := migration.Up(tx); err != nil {
				_ = tx.Rollback()
				return stats, fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
			}

			// Record migration as executed (within the same transaction)
			migrationRecord := &History{
				ID:         lastMigratedID + 1,
				Name:       migration.Name,
				ExecutedAt: time.Now(),
			}
			if _, err := tx.NewInsert().Model(migrationRecord).Exec(ctx); err != nil {
				_ = tx.Rollback()
				return stats, fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
			}
			lastMigratedID++

			// Commit transaction
			if err := tx.Commit(); err != nil {
				return stats, fmt.Errorf("failed to commit migration %s: %w", migration.Name, err)
			}

			stats.TotalSuccess++
			log.Printf("Migration %s completed successfully", migration.Name)
		} else {
			stats.TotalSkipped++
			log.Printf("Migration %s already executed, skipping", migration.Name)
		}
	}

	return stats, nil
}

// RollbackLast rolls back the last executed migration. It returns true if a
// migration was rolled back, or ErrNoMigrationsToRollback when none remain.
//
// The rollback runs inside a transaction: the migration's Down step and the
// removal of its bookkeeping row commit together, or roll back together on
// failure — so a failed rollback never leaves partial schema state.
func RollbackLast(db *bun.DB, migrations []Migration) (bool, error) {
	// Get last executed migration
	ctx := context.Background()
	lastMigration := &History{}
	if err := db.NewSelect().Model(lastMigration).Order("id DESC").Limit(1).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNoMigrationsToRollback
		}
		return false, fmt.Errorf("failed to get last migration: %w", err)
	}

	// Find migration definition
	var migration *Migration
	for _, m := range migrations {
		if m.Name == lastMigration.Name {
			migration = &m
			break
		}
	}

	if migration == nil {
		return false, fmt.Errorf("migration %s not found", lastMigration.Name)
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute rollback inside the transaction
	if err := migration.Down(tx); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("failed to rollback migration %s: %w", migration.Name, err)
	}

	// Remove migration record (within the same transaction)
	if _, err := tx.NewDelete().Model(&History{}).Where("name = ?", migration.Name).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("failed to remove migration record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit rollback: %w", err)
	}

	log.Printf("Migration %s rolled back successfully", migration.Name)
	return true, nil
}

// Reset rolls back all executed migrations and returns the number rolled back.
func Reset(db *bun.DB, migrations []Migration) (int, error) {
	totalRolledBack := 0
	for {
		rolledBack, err := RollbackLast(db, migrations)
		if err != nil {
			if errors.Is(err, ErrNoMigrationsToRollback) {
				break
			}
			return totalRolledBack, err
		}
		if rolledBack {
			totalRolledBack++
		}
	}
	return totalRolledBack, nil
}
