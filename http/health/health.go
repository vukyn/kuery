// Package health answers one question: which build is running right now?
//
// It exists because verifying a deploy used to mean reading the platform's logs
// and spotting an incidental clue — a Fiber banner version that happened to
// change — which only works when a dependency visibly changed AND you have log
// access for that account. This turns it into one curl.
//
// The versions come from debug.ReadBuildInfo, which reports the module versions
// the binary was linked against with no ldflags, no -X injection and no .git in
// the build context. That last point matters: the services' .dockerignore
// excludes .git, so the VCS build settings (vcs.revision / vcs.modified) are
// NOT stamped into a deployed image. They are reported when they happen to be
// there (a local go build, go install) and omitted otherwise — the design does
// not rest on them. Module versions are always there, which is why they are the
// primary answer.
//
// # This must never touch a database, call an external service, or take a lock
//
// Two reasons, neither obvious from the code:
//
//  1. Cost. Several services on this platform run a scale-to-zero deployment in
//     front of a serverless Postgres. A health check that pings the database
//     wakes that Postgres on every call, which is exactly the expense the
//     platform spent effort removing (one service went from a 100% to a 4.3%
//     duty cycle). The database must stay asleep while this endpoint answers.
//
//  2. Meaning. "The process is up" and "its database is reachable" are
//     different questions. Collapsing them makes the endpoint useless for the
//     thing it is for: telling a deploy problem apart from a database problem.
//     A green answer here means the binary you think you shipped is the binary
//     that is running — nothing more, deliberately.
//
// Collector therefore takes no *bun.DB, no connection, no context.Context, and
// Collect takes no arguments — there is nowhere for a dependency to be threaded
// in later without changing an exported signature. TestNewTakesNoDatabaseOrContext
// asserts that shape so the constraint fails a build rather than a code review.
//
// # Having this endpoint does not mean anything may poll it
//
// It is for a human verifying a deploy by hand. Do NOT wire it to an uptime
// monitor, and do NOT add an http_service.checks block in a service's fly.toml
// (rainy's carries an explicit warning against exactly that). Any poller keeps
// the machine awake and silently undoes the
// scale-to-zero cost work described above — the endpoint would then cost more
// than the log-reading it replaced.
package health

import (
	"os"
	"runtime/debug"
	"time"
)

// StatusOK is the only status this package reports. The field is a constant
// rather than a computed value on purpose: a served response already proves the
// process is up, and anything richer would need a dependency to check.
const StatusOK = "ok"

// Fly.io injects these into every machine. They are the cheapest way to tell
// two running machines apart and to name the exact image a deploy produced.
const (
	envMachineID = "FLY_MACHINE_ID"
	envImageRef  = "FLY_IMAGE_REF"
)

// Info is the version report. Every optional field is omitted rather than
// emitted blank, so a local run does not answer with misleading empty strings
// that read like "no revision" instead of "not recorded here".
type Info struct {
	Status    string `json:"status"`
	GoVersion string `json:"go_version"`

	// Modules maps module path to the version the binary was linked against,
	// for the paths the caller asked about. Never nil (a nil map marshals to
	// JSON null, which callers have to special-case); a path that is not in
	// the build's dependency list is absent, not empty.
	Modules map[string]string `json:"modules"`

	// Revision and Modified come from the vcs.revision / vcs.modified build
	// settings and are present only when the toolchain stamped them — see the
	// package doc on .dockerignore. Modified is a pointer because "built from
	// a clean tree" is a meaningful false that must survive omitempty.
	Revision string `json:"revision,omitempty"`
	Modified *bool  `json:"modified,omitempty"`

	// Machine and Image come from the Fly environment, omitted off Fly.
	Machine string `json:"machine,omitempty"`
	Image   string `json:"image,omitempty"`

	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// Collector gathers Info. Build it once at start-up and keep it: the start time
// it reports is the moment it was constructed, which is what makes uptime mean
// "how long has this process been serving" rather than "how long ago did you
// call this".
//
// It holds no I/O handle of any kind — see the package doc.
type Collector struct {
	modulePaths []string
	startedAt   time.Time

	// Seams, injected so the tests can drive time, build info and the
	// environment deterministically. Production always gets the real three.
	now           func() time.Time
	readBuildInfo func() (*debug.BuildInfo, bool)
	lookupEnv     func(string) (string, bool)
}

// New captures the process start time and records which module versions the
// caller cares about. The module list belongs to the caller — kuery must not
// know which dependencies a given service wants to see — so a service passes
// its own, e.g.
//
//	health.New("github.com/vukyn/kuery", "github.com/vukyn/isme", "github.com/gofiber/fiber/v2")
//
// A service's OWN module is deliberately not resolvable: it is BuildInfo.Main,
// not a dep, and its version for a plain go build is the useless "(devel)".
func New(modulePaths ...string) *Collector {
	return newCollector(time.Now, debug.ReadBuildInfo, os.LookupEnv, modulePaths)
}

func newCollector(
	now func() time.Time,
	readBuildInfo func() (*debug.BuildInfo, bool),
	lookupEnv func(string) (string, bool),
	modulePaths []string,
) *Collector {
	return &Collector{
		modulePaths:   modulePaths,
		startedAt:     now(),
		now:           now,
		readBuildInfo: readBuildInfo,
		lookupEnv:     lookupEnv,
	}
}

// Collect builds the report. Pure in-process reads: build info baked into the
// binary, environment variables, and the clock.
func (c *Collector) Collect() Info {
	info := Info{
		Status:        StatusOK,
		Modules:       make(map[string]string, len(c.modulePaths)),
		StartedAt:     c.startedAt,
		UptimeSeconds: int64(c.now().Sub(c.startedAt) / time.Second),
	}

	if buildInfo, ok := c.readBuildInfo(); ok && buildInfo != nil {
		info.GoVersion = buildInfo.GoVersion
		collectModules(info.Modules, buildInfo.Deps, c.modulePaths)
		collectVCS(&info, buildInfo.Settings)
	}

	info.Machine = c.env(envMachineID)
	info.Image = c.env(envImageRef)

	return info
}

// StartedAt is the moment the Collector was constructed.
func (c *Collector) StartedAt() time.Time {
	return c.startedAt
}

// env returns the variable's value, or "" when unset OR set to the empty
// string. Both cases must collapse to "" so the omitempty on Machine/Image
// drops the field instead of reporting a blank that looks like an answer.
func (c *Collector) env(key string) string {
	value, ok := c.lookupEnv(key)
	if !ok {
		return ""
	}
	return value
}

// collectModules resolves only the requested paths. A requested path missing
// from deps is left out of the map entirely — an empty-string version reads as
// a module pinned to the empty version rather than as a module not linked in.
func collectModules(out map[string]string, deps []*debug.Module, modulePaths []string) {
	if len(modulePaths) == 0 || len(deps) == 0 {
		return
	}

	versions := make(map[string]string, len(deps))
	for _, dep := range deps {
		if dep == nil {
			continue
		}
		version := dep.Version
		// A replace directive means the linked code is the replacement's, so
		// report that version rather than the one the requirement named —
		// otherwise a local working copy would be reported under the version it
		// replaced, which is exactly the lie this endpoint exists to prevent.
		// A directory replace reports "(devel)" (verified against a real binary
		// with go version -m), which is the honest answer for a dev build;
		// replace forms that carry no version at all fall to the guard below.
		if dep.Replace != nil {
			version = dep.Replace.Version
		}
		if version == "" {
			continue
		}
		versions[dep.Path] = version
	}

	for _, modulePath := range modulePaths {
		if version, ok := versions[modulePath]; ok {
			out[modulePath] = version
		}
	}
}

// collectVCS fills Revision/Modified only from settings that are actually
// present, leaving both zero when the build was not VCS-stamped.
func collectVCS(info *Info, settings []debug.BuildSetting) {
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			info.Revision = setting.Value
		case "vcs.modified":
			modified := setting.Value == "true"
			info.Modified = &modified
		}
	}
}
