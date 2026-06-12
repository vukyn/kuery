package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/vukyn/kuery/log"
)

// Option configures the Engine at construction.
type Option func(*options)

// options holds the resolved construction settings.
type options struct {
	// location, when non-nil, is passed to gocron via WithLocation. When nil,
	// NO location option is passed so gocron uses its default (local time) — this
	// is the isme-parity behavior and must not silently become UTC.
	location *time.Location
}

// WithLocation sets the time zone gocron evaluates schedules in. medioa2 passes
// time.UTC; isme omits this so the engine runs in the process's local time
// (matching the original engine, which never set a location).
func WithLocation(location *time.Location) Option {
	return func(o *options) {
		o.location = location
	}
}

// Engine owns a gocron scheduler and a keyed registry of named jobs. It is an
// application-scoped singleton (one per process), DB-agnostic: job bodies and
// schedule persistence are supplied by the service. *Engine implements
// IReloader.
type Engine struct {
	sched   gocron.Scheduler
	enabled bool

	mu       sync.Mutex
	registry map[JobKey]Job
	jobs     map[JobKey]uuid.UUID
	started  bool
	stopped  bool
}

// New constructs the engine. enabled is the master kill-switch (typically env
// SCHEDULER_ENABLED): when false the engine never installs any job, but the
// underlying gocron scheduler is still started so a later Reload can add a job
// without a process restart.
func New(enabled bool, opts ...Option) (*Engine, error) {
	resolved := &options{}
	for _, opt := range opts {
		opt(resolved)
	}

	var schedulerOpts []gocron.SchedulerOption
	if resolved.location != nil {
		schedulerOpts = append(schedulerOpts, gocron.WithLocation(resolved.location))
	}

	sched, err := gocron.NewScheduler(schedulerOpts...)
	if err != nil {
		return nil, err
	}
	return &Engine{
		sched:    sched,
		enabled:  enabled,
		registry: make(map[JobKey]Job),
		jobs:     make(map[JobKey]uuid.UUID),
	}, nil
}

// Register adds a job to the registry. Call before Start. Registering the same
// key twice is last-wins. The job is not installed until Start (or a later
// Reload) — registration only records the closure to run.
func (e *Engine) Register(job Job) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registry[job.Key] = job
}

// Start starts the underlying gocron scheduler and, if the master kill-switch
// allows, loads and installs each registered job. The scheduler is always
// started so a later Reload can add a job without a restart. Each job loads and
// installs independently — one job's failure logs and continues, never blocking
// the others (failure isolation).
func (e *Engine) Start(ctx context.Context, provider ScheduleProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.sched.Start()
	e.started = true

	if !e.enabled {
		log.New().Info("Scheduler disabled by master kill-switch")
		return
	}

	for key := range e.registry {
		enabled, schedule, err := provider.Load(ctx, key)
		if err != nil {
			log.New().Errorf("Scheduler: failed to load config for %q: %v", key, err)
			continue
		}
		if !enabled {
			continue
		}
		if err := e.applyJob(key, schedule); err != nil {
			log.New().Errorf("Scheduler: failed to install job %q: %v", key, err)
			continue
		}
		log.New().Infof("Scheduler installed job %q", key)
	}
}

// Reload re-applies the schedule for the named job live. It removes the current
// job (if any) and, when enabled (and the master kill-switch allows), installs a
// fresh job for the given schedule. This is the live-update path used by a
// settings usecase after a config change — no process restart required.
func (e *Engine) Reload(ctx context.Context, key JobKey, enabled bool, schedule Schedule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return nil
	}

	e.removeJob(key)

	if !e.enabled || !enabled {
		log.New().Infof("Scheduler reloaded %q: no active job", key)
		return nil
	}
	if err := e.applyJob(key, schedule); err != nil {
		return err
	}
	log.New().Infof("Scheduler reloaded %q", key)
	return nil
}

// Stop shuts the scheduler down. It is idempotent and bounded by gocron's own
// shutdown — it must never block graceful shutdown indefinitely.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped || !e.started {
		e.stopped = true
		return
	}
	if err := e.sched.Shutdown(); err != nil {
		log.New().Errorf("Scheduler: shutdown error: %v", err)
	}
	e.stopped = true
}

// applyJob installs a new job for the given key, dispatching to the registered
// closure. Callers must hold e.mu. A key with no registered job is a no-op.
func (e *Engine) applyJob(key JobKey, schedule Schedule) error {
	job, ok := e.registry[key]
	if !ok {
		return nil
	}

	definition, err := schedule.jobDefinition()
	if err != nil {
		return err
	}

	run := job.Run
	task := gocron.NewTask(func() {
		if err := run(context.Background()); err != nil {
			log.New().Errorf("Scheduler: job %q run failed: %v", key, err)
		}
	})

	created, err := e.sched.NewJob(
		definition,
		task,
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	e.jobs[key] = created.ID()
	return nil
}

// removeJob removes the installed job for the given key if present. Callers must
// hold e.mu.
func (e *Engine) removeJob(key JobKey) {
	jobID, ok := e.jobs[key]
	if !ok {
		return
	}
	if err := e.sched.RemoveJob(jobID); err != nil {
		log.New().Errorf("Scheduler: failed to remove job %q: %v", key, err)
	}
	delete(e.jobs, key)
}
