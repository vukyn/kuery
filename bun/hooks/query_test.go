package bun

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// captureLogger is a log.SimpleLogger that keeps what it was told instead of
// printing it. log/ has no such fake — its own test exercises Init, not a
// logger — so it lives here, next to the only assertion that needs it.
type captureLogger struct {
	mutex sync.Mutex
	lines []string
}

func (c *captureLogger) record(line string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.lines = append(c.lines, line)
}

func (c *captureLogger) output() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return strings.Join(c.lines, "\n")
}

func (c *captureLogger) Info(msg string)                { c.record(msg) }
func (c *captureLogger) Infof(format string, v ...any)  { c.record(fmt.Sprintf(format, v...)) }
func (c *captureLogger) Error(msg string)               { c.record(msg) }
func (c *captureLogger) Errorf(format string, v ...any) { c.record(fmt.Sprintf(format, v...)) }
func (c *captureLogger) Debug(msg string)               { c.record(msg) }
func (c *captureLogger) Debugf(msg string, args ...any) { c.record(fmt.Sprintf(msg, args...)) }
func (c *captureLogger) Warn(msg string)                { c.record(msg) }
func (c *captureLogger) Warnf(msg string, args ...any)  { c.record(fmt.Sprintf(msg, args...)) }
func (c *captureLogger) Fatal(msg string, err error)    { c.record(msg) }
func (c *captureLogger) Fatalf(msg string, args ...any) { c.record(fmt.Sprintf(msg, args...)) }
func (c *captureLogger) Panic(msg string)               { c.record(msg) }
func (c *captureLogger) Panicf(msg string, args ...any) { c.record(fmt.Sprintf(msg, args...)) }

// theSecret stands in for the things these services actually put in query
// arguments — argon2id hashes, refresh tokens, session ids, OAuth codes, API
// keys. Naming it keeps the literal out of the assertion lines.
const theSecret = "argon2id-hash-and-refresh-token-DO-NOT-LOG"

// TestQueryHookLogsNoArgumentValues is the regression guard for the credential
// leak: the hook must log the SQL template and nothing that came from an
// argument.
//
// It drives a REAL query through bun rather than hand-building a QueryEvent,
// because the whole bug was a wrong belief about which field holds what. A
// fabricated event would just restate that belief; a real one proves that bun's
// Query really does carry the value inlined and that QueryTemplate really does
// not — so the test still means something if bun changes its mind.
//
// Mutation: put event.Query back in place of event.QueryTemplate and this fails.
func TestQueryHookLogsNoArgumentValues(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:hooktest?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqldb.Close()

	capture := &captureLogger{}
	database := bun.NewDB(sqldb, sqlitedialect.New())
	defer database.Close()
	database.AddQueryHook(NewQueryHook(capture))

	ctx := context.Background()
	if _, err := database.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS accounts (id INTEGER PRIMARY KEY, secret TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO accounts (secret) VALUES (?)", theSecret); err != nil {
		t.Fatalf("insert: %v", err)
	}

	logged := capture.output()
	if logged == "" {
		t.Fatal("the hook logged nothing — the assertions below would pass vacuously")
	}
	if strings.Contains(logged, theSecret) {
		t.Fatalf("the argument value reached the log:\n%s", logged)
	}
	if !strings.Contains(logged, "INSERT INTO accounts") {
		t.Fatalf("the statement itself must still be logged, or the hook is useless:\n%s", logged)
	}
	// The placeholder is what proves the TEMPLATE was logged rather than a
	// formatted query that happened not to contain this particular value.
	if !strings.Contains(logged, "?") {
		t.Fatalf("expected the placeholder to survive into the log:\n%s", logged)
	}
	// Timing is the other half of what this hook is for and must not be lost.
	if !strings.Contains(logged, "[") {
		t.Fatalf("expected the query duration to still be logged:\n%s", logged)
	}
}

// TestQueryEventFieldsStillMeanWhatWeThink pins the bun contract the fix rests
// on. If a future bun release starts inlining values into QueryTemplate, the
// leak returns with no change to kuery and nothing else would notice.
func TestQueryEventFieldsStillMeanWhatWeThink(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:hookcontract?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqldb.Close()

	database := bun.NewDB(sqldb, sqlitedialect.New())
	defer database.Close()

	var observed *bun.QueryEvent
	database.AddQueryHook(&eventSpy{onAfter: func(event *bun.QueryEvent) {
		if strings.Contains(event.QueryTemplate, "INSERT") {
			observed = event
		}
	}})

	ctx := context.Background()
	if _, err := database.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS accounts (id INTEGER PRIMARY KEY, secret TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO accounts (secret) VALUES (?)", theSecret); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if observed == nil {
		t.Fatal("no INSERT event observed")
	}
	if !strings.Contains(observed.Query, theSecret) {
		t.Errorf("bun changed: QueryEvent.Query no longer inlines values — re-check whether this hook is still choosing the right field")
	}
	if strings.Contains(observed.QueryTemplate, theSecret) {
		t.Errorf("bun changed: QueryEvent.QueryTemplate now inlines values — the hook is leaking again")
	}
}

type eventSpy struct {
	onAfter func(*bun.QueryEvent)
}

func (s *eventSpy) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (s *eventSpy) AfterQuery(ctx context.Context, event *bun.QueryEvent) { s.onAfter(event) }
