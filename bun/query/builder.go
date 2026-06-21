package query

import (
	"strings"

	pkgBase "github.com/vukyn/kuery/http/base"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

func SelectWithPagination(query *bun.SelectQuery, paging pkgBase.Pagination, defaultSort string) *bun.SelectQuery {
	if paging.SortBy != "" {
		if strings.ToLower(paging.SortOrder) == "asc" {
			query = query.Order(paging.SortBy + " ASC")
		} else {
			query = query.Order(paging.SortBy + " DESC")
		}
	} else {
		// defaultSort is a developer-supplied raw ORDER BY expression (may span
		// multiple columns / expressions like "position IS NULL, position ASC,
		// created_at DESC"). Use OrderExpr so bun does NOT parse it as a single
		// "column direction" pair — Order() would reject expression parts with
		// an slog "unsupported sort direction" warning and drop the sort.
		query = query.OrderExpr(defaultSort)
	}

	if paging.GetLimit() > 0 {
		query = query.Limit(paging.GetLimit())
	}

	if paging.GetOffset() > 0 {
		query = query.Offset(paging.GetOffset())
	}
	return query
}

// ILike returns a dialect-correct case-insensitive LIKE predicate fragment for
// the given column, portable across SQLite and Postgres, for use in a bun
// .Where(...) clause. The caller supplies the bound pattern argument:
//
//	db.NewSelect().Model(&rows).Where(query.ILike(db, "name_search"), pattern)
//
// On Postgres LIKE is case-SENSITIVE, so this emits `column ILIKE ?`; on SQLite
// LIKE is already case-insensitive for ASCII, so it emits `column LIKE ?`.
//
// column is a trusted developer-supplied identifier (never user input), so it is
// concatenated directly. The pattern value stays a bound `?` placeholder.
func ILike(db bun.IDB, column string) string {
	if db.Dialect().Name() == dialect.PG {
		return column + " ILIKE ?"
	}
	return column + " LIKE ?"
}

// LowerLike returns a case-insensitive LIKE predicate fragment that folds both
// the column and the bound pattern to lower case, portable across SQLite and
// Postgres without needing a dialect handle:
//
//	db.NewSelect().Model(&rows).Where(query.LowerLike("country"), pattern)
//
// Prefer ILike when a *bun.DB is in scope; use LowerLike for exact-ish lower
// case matching or where no dialect handle is readily available.
//
// column is a trusted developer-supplied identifier (never user input), so it is
// concatenated directly. The pattern value stays a bound `?` placeholder.
func LowerLike(column string) string {
	return "LOWER(" + column + ") LIKE LOWER(?)"
}

// BoolToInt converts a boolean value to integer for SQLite compatibility
// SQLite stores booleans as integers (0 for false, 1 for true)
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// BoolPtrToInt converts a boolean pointer to integer for SQLite compatibility
// Returns 0 if the pointer is nil
func BoolPtrToInt(b *bool) int {
	if b == nil {
		return 0
	}
	return BoolToInt(*b)
}
