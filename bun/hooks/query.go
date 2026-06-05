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
	// q.logger.Debugf("BeforeQuery: %s", string(event.Query))
	return ctx
}

func (q *queryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	q.logger.Infof("[%s] %s", time.Since(event.StartTime), event.Query)
}
