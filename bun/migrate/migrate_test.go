package migrate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// newTestDB opens a fresh file-backed SQLite database (no CGO, via sqliteshim)
// under the test's temp dir. A file rather than ":memory:" is used so the schema
// survives across pooled connections for the duration of the test.
func newTestDB(t *testing.T) *bun.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "migrate_test.db")
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Single connection keeps the pool from racing on the shared file.
	sqldb.SetMaxOpenConns(1)

	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

// countRows reports how many rows exist in the given table.
func countRows(t *testing.T, db *bun.DB, table string) int {
	t.Helper()
	count, err := db.NewSelect().Table(table).Count(context.Background())
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// makeMigrations builds a set of migrations that create and drop simple tables,
// recording which Up/Down steps were actually invoked.
func makeMigrations(applied map[string]int, rolledBack map[string]int) []Migration {
	mk := func(name, table string) Migration {
		return Migration{
			Name: name,
			Up: func(db bun.IDB) error {
				applied[name]++
				_, err := db.NewRaw("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY)").Exec(context.Background())
				return err
			},
			Down: func(db bun.IDB) error {
				rolledBack[name]++
				_, err := db.NewRaw("DROP TABLE " + table).Exec(context.Background())
				return err
			},
		}
	}
	return []Migration{
		mk("001_create_alpha", "alpha"),
		mk("002_create_beta", "beta"),
		mk("003_create_gamma", "gamma"),
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name             string
		preRun           bool // run migrations once before the measured run
		wantSuccess      int
		wantSkipped      int
		wantHistoryCount int
	}{
		{
			name:             "applies all pending migrations",
			preRun:           false,
			wantSuccess:      3,
			wantSkipped:      0,
			wantHistoryCount: 3,
		},
		{
			name:             "second run skips already-executed migrations",
			preRun:           true,
			wantSuccess:      0,
			wantSkipped:      3,
			wantHistoryCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			applied := map[string]int{}
			rolledBack := map[string]int{}

			if tt.preRun {
				if _, err := Run(db, makeMigrations(applied, rolledBack)); err != nil {
					t.Fatalf("pre-run Run() error: %v", err)
				}
			}

			// Fresh counters for the measured run.
			applied = map[string]int{}
			stats, err := Run(db, makeMigrations(applied, rolledBack))
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}
			if stats.TotalSuccess != tt.wantSuccess {
				t.Errorf("TotalSuccess = %d, want %d", stats.TotalSuccess, tt.wantSuccess)
			}
			if stats.TotalSkipped != tt.wantSkipped {
				t.Errorf("TotalSkipped = %d, want %d", stats.TotalSkipped, tt.wantSkipped)
			}
			if got := countRows(t, db, "migrations"); got != tt.wantHistoryCount {
				t.Errorf("migrations rows = %d, want %d", got, tt.wantHistoryCount)
			}
		})
	}
}

func TestRunAssignsSequentialIDs(t *testing.T) {
	db := newTestDB(t)
	applied := map[string]int{}
	rolledBack := map[string]int{}

	if _, err := Run(db, makeMigrations(applied, rolledBack)); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	var history []History
	if err := db.NewSelect().Model(&history).Order("id ASC").Scan(context.Background()); err != nil {
		t.Fatalf("scan history: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("history len = %d, want 3", len(history))
	}
	for i, h := range history {
		if h.ID != int64(i+1) {
			t.Errorf("history[%d].ID = %d, want %d", i, h.ID, i+1)
		}
	}
}

func TestRollbackLast(t *testing.T) {
	db := newTestDB(t)
	applied := map[string]int{}
	rolledBack := map[string]int{}
	migrations := makeMigrations(applied, rolledBack)

	if _, err := Run(db, migrations); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Roll back the three migrations one at a time, last-in first-out.
	wantOrder := []string{"003_create_gamma", "002_create_beta", "001_create_alpha"}
	for i, wantName := range wantOrder {
		ok, err := RollbackLast(db, migrations)
		if err != nil {
			t.Fatalf("RollbackLast() #%d unexpected error: %v", i, err)
		}
		if !ok {
			t.Fatalf("RollbackLast() #%d returned false, want true", i)
		}
		if rolledBack[wantName] != 1 {
			t.Errorf("expected Down of %s to run exactly once, got %d", wantName, rolledBack[wantName])
		}
		if got := countRows(t, db, "migrations"); got != len(wantOrder)-i-1 {
			t.Errorf("after rollback #%d migrations rows = %d, want %d", i, got, len(wantOrder)-i-1)
		}
	}

	// Nothing left to roll back.
	ok, err := RollbackLast(db, migrations)
	if !errors.Is(err, ErrNoMigrationsToRollback) {
		t.Fatalf("RollbackLast() on empty = %v, want ErrNoMigrationsToRollback", err)
	}
	if ok {
		t.Fatalf("RollbackLast() on empty returned true, want false")
	}
}

func TestRollbackLastMissingDefinition(t *testing.T) {
	db := newTestDB(t)
	applied := map[string]int{}
	rolledBack := map[string]int{}
	migrations := makeMigrations(applied, rolledBack)

	if _, err := Run(db, migrations); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Roll back with a migration set that no longer defines the last-run
	// migration — the runner cannot find its Down step and must error.
	orphan := migrations[:2]
	if _, err := RollbackLast(db, orphan); err == nil {
		t.Fatalf("RollbackLast() expected error for missing migration definition, got nil")
	}
}
