package bun

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"github.com/vukyn/kuery/log"
)

type queryHook struct {
	logger log.SimpleLogger
}

func NewQueryHook(logger log.SimpleLogger) bun.QueryHook {
	return &queryHook{logger: logger}
}

func (q *queryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery logs the query TEMPLATE, never the executed query.
//
// bun exposes three separate fields on a QueryEvent: Query is the formatted SQL
// with every argument value already inlined, QueryTemplate keeps the
// placeholders, and QueryArgs holds the values on their own. Logging Query puts
// whatever the statement carried into the log verbatim — password hashes,
// tokens, session identifiers, keys — so this logs QueryTemplate and neither of
// the other two.
//
// Scope, so the reasoning is not overstated later: this is an Info call, and the
// deployed services run at warn, so nothing was being written in production. The
// leak was on developer machines, which run at debug/info. The reason it still
// had to change is that a single config value was the only thing separating a
// production log from a credential dump — anyone dropping the level to debug to
// chase a bug for an afternoon would have got one, and a service that never sets
// the level at all inherits whatever the default is.
func (q *queryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	q.logger.Infof("[%s] %s", time.Since(event.StartTime), event.QueryTemplate)
}
