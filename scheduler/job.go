// Package scheduler is a DB-agnostic gocron wrapper for running named,
// schedule-driven background jobs in the pet-platform services.
//
// It generalizes the engine that previously lived inside isme's
// internal/scheduler: a master kill-switch, a keyed job registry, failure-
// isolated per-job loading at start, idempotent shutdown, and live schedule
// reloads — without any knowledge of a specific service's persistence (Bun/
// SQLite, Mongo, etc.) or job bodies. Each service supplies its own job
// closures (Job.Run), its own schedule persistence (ScheduleProvider), and its
// own result recording (done inside Run).
//
// The two schedule modes — raw cron (isme) and friendly preset/daily-weekly
// (medioa2) — share one engine via the opaque Schedule value (see schedule.go),
// so neither service needs a local engine copy.
package scheduler

import "context"

// JobKey identifies a named scheduled job in the keyed registry. Services
// define their own key constants (typically in a settings entity package, the
// single source of truth shared with the repository, usecase and migration) and
// convert them to JobKey at registration.
type JobKey string

// Job is a named unit of scheduled work. Run is a service-supplied closure: it
// reads any params it needs fresh, performs the work, and records its own
// result via the service's persistence. The engine is result-agnostic — it
// never inspects Run's return beyond logging an error.
type Job struct {
	// Key is the registry key the job is installed under.
	Key JobKey
	// Run is the work performed on each fire. It must not panic; recoverable
	// failures should be returned (or logged + swallowed) by the closure so a
	// single bad run never crashes the process or kills the schedule.
	Run func(ctx context.Context) error
}

// IReloader is the seam a settings usecase depends on to re-apply a changed
// schedule live, without restarting the process. It lives in this package so
// the usecase imports the scheduler (one direction only) and no import cycle
// forms. *Engine implements IReloader.
type IReloader interface {
	// Reload re-applies the schedule for the named job: it removes any current
	// job and, when enabled (and the engine's master kill-switch allows),
	// installs a fresh job for the given schedule. This is the live-update path —
	// the caller already holds the new values, so the ScheduleProvider is not
	// consulted.
	Reload(ctx context.Context, key JobKey, enabled bool, schedule Schedule) error
}

// ScheduleProvider is the engine's initial-load path: at Start, the engine asks
// the provider for each registered job's persisted schedule. The service
// implements Load over its own store (Bun, Mongo, …). Reload is the separate
// live-update path and does not go through the provider.
type ScheduleProvider interface {
	// Load returns the persisted enabled flag and schedule for the named job. A
	// disabled job (or one with an effectively empty schedule) is simply not
	// installed by the engine.
	Load(ctx context.Context, key JobKey) (enabled bool, schedule Schedule, err error)
}
