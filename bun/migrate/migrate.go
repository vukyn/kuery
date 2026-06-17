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
)

// ErrNoMigrationsToRollback is returned by RollbackLast when there are no
// executed migrations left to roll back. Detect it with errors.Is.
var ErrNoMigrationsToRollback = errors.New("no migrations to rollback")

// Migration represents a single database migration with its up/down steps.
type Migration struct {
	Name string
	Up   func(*bun.DB) error
	Down func(*bun.DB) error
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

// Run executes all pending migrations against the given database.
func Run(db *bun.DB, migrations []Migration) (Stats, error) {
	stats := Stats{}

	// Create migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return stats, fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get executed migrations
	var executedMigrations []History
	ctx := context.Background()
	if err := db.NewSelect().Table("migrations").Scan(ctx, &executedMigrations); err != nil {
		return stats, fmt.Errorf("failed to get executed migrations: %w", err)
	}

	// Execute pending migrations
	lastMigratedID := int64(0)
	if len(executedMigrations) > 0 {
		lastMigratedID = executedMigrations[len(executedMigrations)-1].ID
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

			// Execute migration
			if err := migration.Up(db); err != nil {
				_ = tx.Rollback()
				return stats, fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
			}

			// Record migration as executed
			migrationRecord := &History{
				ID:         lastMigratedID + 1,
				Name:       migration.Name,
				ExecutedAt: time.Now(),
			}
			if _, err := db.NewInsert().Model(migrationRecord).Exec(ctx); err != nil {
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

	// Execute rollback
	if err := migration.Down(db); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("failed to rollback migration %s: %w", migration.Name, err)
	}

	// Remove migration record
	if _, err := db.NewDelete().Model(&History{}).Where("name = ?", migration.Name).Exec(ctx); err != nil {
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
