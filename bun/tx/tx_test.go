package tx

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// openDB opens a private in-memory SQLite database with one table, shared by
// name across connections. Two connections, not one: a nested RunInTx that
// wrongly opened a second transaction would wait forever for a single
// connection, turning a failure into a hang.
func openDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqldb.SetMaxOpenConns(2)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE items (name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func insert(ctx context.Context, db *bun.DB, name string) error {
	_, err := Conn(ctx, db).NewRaw(`INSERT INTO items (name) VALUES (?)`, name).Exec(ctx)
	return err
}

func count(t *testing.T, db *bun.DB) int {
	t.Helper()
	var n int
	if err := db.NewRaw(`SELECT count(*) FROM items`).Scan(context.Background(), &n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestConnOutsideATransactionIsThePlainDB(t *testing.T) {
	db := openDB(t)
	if got := Conn(context.Background(), db); got != bun.IDB(db) {
		t.Fatalf("Conn without a transaction = %T, want the *bun.DB", got)
	}
}

func TestConnInsideRunInTxIsTheTransaction(t *testing.T) {
	db := openDB(t)
	err := NewRunner(db).RunInTx(context.Background(), func(ctx context.Context) error {
		if _, ok := Conn(ctx, db).(bun.Tx); !ok {
			t.Fatalf("Conn inside RunInTx = %T, want bun.Tx", Conn(ctx, db))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunInTx: %v", err)
	}
}

func TestRunInTxCommitsOnNil(t *testing.T) {
	db := openDB(t)
	err := NewRunner(db).RunInTx(context.Background(), func(ctx context.Context) error {
		if err := insert(ctx, db, "a"); err != nil {
			return err
		}
		return insert(ctx, db, "b")
	})
	if err != nil {
		t.Fatalf("RunInTx: %v", err)
	}
	if got := count(t, db); got != 2 {
		t.Fatalf("rows after commit = %d, want 2", got)
	}
}

func TestRunInTxRollsBackEveryWriteOnError(t *testing.T) {
	db := openDB(t)
	boom := errors.New("boom")
	err := NewRunner(db).RunInTx(context.Background(), func(ctx context.Context) error {
		if err := insert(ctx, db, "a"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("RunInTx error = %v, want %v", err, boom)
	}
	if got := count(t, db); got != 0 {
		t.Fatalf("rows after rollback = %d, want 0", got)
	}
}

// A nested RunInTx joins the outer transaction: when the outer one rolls
// back, the inner write goes with it even though the inner call returned nil.
func TestNestedRunInTxJoinsTheOuterTransaction(t *testing.T) {
	db := openDB(t)
	runner := NewRunner(db)
	boom := errors.New("boom")
	err := runner.RunInTx(context.Background(), func(outer context.Context) error {
		if err := runner.RunInTx(outer, func(inner context.Context) error {
			if Conn(inner, db) != Conn(outer, db) {
				t.Fatal("nested RunInTx opened a second transaction")
			}
			return insert(inner, db, "inner")
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("RunInTx error = %v, want %v", err, boom)
	}
	if got := count(t, db); got != 0 {
		t.Fatalf("inner write outlived the outer rollback: %d row(s)", got)
	}
}
