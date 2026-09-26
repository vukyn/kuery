// Package tx lets one usecase run writes owned by SEVERAL repositories inside
// one database transaction, without a `bun.Tx` ever appearing on a usecase or
// consumer-side interface.
//
// The transaction travels in the context. A repository method that may take
// part in one reads its handle through Conn instead of using its `*bun.DB`
// directly; outside a transaction Conn answers the plain DB, so the method
// behaves exactly as before for every other caller. Only the methods a
// transaction actually spans need Conn — the rest stay on `r.db`.
//
// ⚠️ A repository method that ignores Conn and writes through `r.db` inside a
// Runner call escapes the transaction silently: its write commits even when
// the transaction rolls back. Only a rollback test against a real database
// catches that.
package tx

import (
	"context"

	"github.com/uptrace/bun"
)

type txKey struct{}

// Conn returns the transaction carried by ctx, or db when there is none.
func Conn(ctx context.Context, db *bun.DB) bun.IDB {
	if tx, ok := ctx.Value(txKey{}).(bun.Tx); ok {
		return tx
	}
	return db
}

// Runner opens a transaction and hands fn a context carrying it.
type Runner struct {
	db *bun.DB
}

func NewRunner(db *bun.DB) *Runner {
	return &Runner{db: db}
}

// RunInTx commits when fn returns nil and rolls back otherwise. Called with a
// context that already carries a transaction, it joins that transaction rather
// than opening a second, independent one — an inner commit must never be able
// to outlive an outer rollback.
func (r *Runner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(bun.Tx); ok {
		return fn(ctx)
	}
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}
