package query

import (
	"strings"

	pkgBase "github.com/vukyn/kuery/http/base"

	"github.com/uptrace/bun"
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
