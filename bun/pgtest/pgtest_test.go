package pgtest

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/uptrace/bun"
)

// migrateCalls counts how often the template migration ran in this process.
var migrateCalls atomic.Int32

func TestMain(m *testing.M) {
	os.Exit(Run(m, Options{Migrate: migrateWidgets}))
}

func migrateWidgets(database *bun.DB) error {
	migrateCalls.Add(1)
	_, err := database.ExecContext(context.Background(),
		"CREATE TABLE widgets (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)")
	return err
}

func countRows(t *testing.T, database *bun.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return count
}

func currentDatabase(t *testing.T, database *bun.DB) string {
	t.Helper()
	var name string
	if err := database.QueryRowContext(context.Background(), "SELECT current_database()").Scan(&name); err != nil {
		t.Fatalf("current_database: %v", err)
	}
	return name
}

func TestDBIsolation(t *testing.T) {
	first := DB(t)
	second := DB(t)

	if currentDatabase(t, first) == currentDatabase(t, second) {
		t.Fatal("two DB calls returned the same database")
	}
	if _, err := first.ExecContext(context.Background(), "INSERT INTO widgets (name) VALUES ('a')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := countRows(t, first, "SELECT count(*) FROM widgets"); got != 1 {
		t.Fatalf("first database widgets = %d, want 1", got)
	}
	if got := countRows(t, second, "SELECT count(*) FROM widgets"); got != 0 {
		t.Fatalf("second database widgets = %d, want 0 (rows leaked between clones)", got)
	}
}

func TestMigrateRunsOncePerProcess(t *testing.T) {
	for range 3 {
		database := DB(t)
		// The schema must be present in every clone, not only the first.
		if got := countRows(t, database,
			"SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'widgets'"); got != 1 {
			t.Fatalf("widgets table missing from clone %s", currentDatabase(t, database))
		}
	}
	if got := migrateCalls.Load(); got != 1 {
		t.Fatalf("Migrate ran %d times, want 1", got)
	}
}

func TestCloneDroppedAfterCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("pgtest: needs Docker")
	}
	var cloneName string
	t.Run("inner", func(t *testing.T) {
		cloneName = currentDatabase(t, DB(t))
	})
	if cloneName == "" {
		t.Fatal("inner subtest did not record a database name")
	}

	sibling := DB(t)
	if got := countRows(t, sibling, "SELECT count(*) FROM pg_database WHERE datname = ?", cloneName); got != 0 {
		t.Fatalf("database %s still exists after its test finished", cloneName)
	}
	// The template itself must survive every cleanup.
	if got := countRows(t, sibling, "SELECT count(*) FROM pg_database WHERE datname = ?", templateDatabase); got != 1 {
		t.Fatalf("template database %s missing", templateDatabase)
	}
}

func TestEmptyDBHasNoTables(t *testing.T) {
	database := EmptyDB(t)
	if got := countRows(t, database,
		"SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'"); got != 0 {
		t.Fatalf("EmptyDB has %d tables in public, want 0", got)
	}
}

func TestDatabaseName(t *testing.T) {
	cases := []struct {
		name     string
		testName string
		counter  int
		want     string
	}{
		{name: "lowercases and replaces separators", testName: "TestFoo/Sub-Case #1", counter: 7, want: "t_testfoo_sub_case__1_7"},
		{name: "non-ascii becomes underscore", testName: "TestĐạt", counter: 1, want: "t_test__t_1"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := databaseName(testCase.testName, testCase.counter)
			if got != testCase.want {
				t.Fatalf("databaseName(%q) = %q, want %q", testCase.testName, got, testCase.want)
			}
			assertValidName(t, got)
		})
	}

	long := databaseName("Test"+strings.Repeat("X", 200), 123456)
	assertValidName(t, long)
	if !strings.HasSuffix(long, "_123456") {
		t.Fatalf("truncation dropped the counter suffix: %q", long)
	}
}

func assertValidName(t *testing.T, name string) {
	t.Helper()
	if len(name) > maxIdentifierLength {
		t.Fatalf("%q is %d bytes, max %d", name, len(name), maxIdentifierLength)
	}
	for _, character := range name {
		valid := character == '_' || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
		if !valid {
			t.Fatalf("%q contains invalid character %q", name, character)
		}
	}
}
