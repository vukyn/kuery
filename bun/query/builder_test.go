package query

import (
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestILike(t *testing.T) {
	pgDB := bun.NewDB(nil, pgdialect.New())
	sqliteDB := bun.NewDB(nil, sqlitedialect.New())

	if got, want := ILike(pgDB, "name_search"), "name_search ILIKE ?"; got != want {
		t.Errorf("ILike(pg) = %q, want %q", got, want)
	}
	if got, want := ILike(sqliteDB, "name_search"), "name_search LIKE ?"; got != want {
		t.Errorf("ILike(sqlite) = %q, want %q", got, want)
	}
}

func TestLowerLike(t *testing.T) {
	if got, want := LowerLike("country"), "LOWER(country) LIKE LOWER(?)"; got != want {
		t.Errorf("LowerLike() = %q, want %q", got, want)
	}
}
