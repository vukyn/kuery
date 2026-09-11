package health

import (
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// Module paths used as fixture data. Named rather than inlined so a reader can
// tell a requested path from a linked one at the call site.
const (
	kueryModule  = "github.com/vukyn/kuery"
	ismeModule   = "github.com/vukyn/isme"
	fiberModule  = "github.com/gofiber/fiber/v2"
	absentModule = "github.com/vukyn/not-linked-in"

	kueryVersion = "v1.60.0"
	ismeVersion  = "v1.3.0"
	fiberVersion = "v2.52.14"
)

// fixtureBuildInfo stands in for the real build info on purpose. Verified by
// probe: under `go test` debug.ReadBuildInfo() returns ok=true but Deps is
// EMPTY (the test binary's package here depends on nothing outside stdlib) and
// no vcs.* settings are stamped. Asserting module lookup against the real build
// info would therefore assert nothing at all — it would pass with a lookup that
// always returns nothing. TestFixtureIsNotEmpty guards the fixture itself, and
// TestRealBuildInfoIsReadable covers the one field the runtime really provides.
func fixtureBuildInfo() (*debug.BuildInfo, bool) {
	return &debug.BuildInfo{
		GoVersion: "go1.27.1",
		Main:      debug.Module{Path: kueryModule, Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: kueryModule, Version: kueryVersion},
			{Path: ismeModule, Version: ismeVersion},
			{Path: fiberModule, Version: fiberVersion},
		},
	}, true
}

func fixtureBuildInfoWithVCS(revision string, modified string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		buildInfo, _ := fixtureBuildInfo()
		buildInfo.Settings = []debug.BuildSetting{
			{Key: "-compiler", Value: "gc"},
			{Key: "vcs.revision", Value: revision},
			{Key: "vcs.modified", Value: modified},
		}
		return buildInfo, true
	}
}

func noEnv(string) (string, bool) { return "", false }

// fakeClock is a manually advanced clock, so uptime is asserted without a sleep.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func collectJSON(t *testing.T, info Info) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal Info: %v", err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal Info: %v", err)
	}
	return fields
}

// TestFixtureIsNotEmpty makes the module tests incapable of passing trivially:
// if the fixture ever loses its deps, this fails first and says why, instead of
// the lookup tests quietly asserting over an empty list.
func TestFixtureIsNotEmpty(t *testing.T) {
	buildInfo, ok := fixtureBuildInfo()
	if !ok || buildInfo == nil {
		t.Fatal("fixture build info must be present; module tests are meaningless without it")
	}
	if len(buildInfo.Deps) < 2 {
		t.Fatalf("fixture must carry at least 2 deps, got %d — module lookup tests would pass on an empty list", len(buildInfo.Deps))
	}
	for _, dep := range buildInfo.Deps {
		if dep.Version == "" {
			t.Fatalf("fixture dep %q has no version; the version-vs-path assertion would be untestable", dep.Path)
		}
		if dep.Version == dep.Path {
			t.Fatalf("fixture dep %q has version equal to its path; a lookup returning the path would pass wrongly", dep.Path)
		}
	}
}

// TestRealBuildInfoIsReadable exercises the real debug.ReadBuildInfo path that
// production uses. It fails loudly rather than skipping when build info is
// unavailable, because "no build info" means the endpoint answers nothing.
func TestRealBuildInfoIsReadable(t *testing.T) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok || buildInfo == nil {
		t.Fatal("debug.ReadBuildInfo() reported ok=false: this build cannot report versions at all, so the health endpoint would answer with an empty report")
	}
	if buildInfo.GoVersion == "" {
		t.Fatal("real build info carries no GoVersion")
	}

	got := New(kueryModule).Collect()
	if got.GoVersion != buildInfo.GoVersion {
		t.Fatalf("GoVersion = %q, want %q from real build info", got.GoVersion, buildInfo.GoVersion)
	}
	if got.GoVersion != runtime.Version() {
		t.Fatalf("GoVersion = %q, want runtime.Version() %q", got.GoVersion, runtime.Version())
	}
	if got.Status != StatusOK {
		t.Fatalf("Status = %q, want %q", got.Status, StatusOK)
	}
	if got.Modules == nil {
		t.Fatal("Modules must never be nil (a nil map marshals to JSON null)")
	}

	// Not an assertion, a warning: this is exactly why the module tests use a
	// fixture. A test binary for a stdlib-only package links no deps.
	if len(buildInfo.Deps) == 0 {
		t.Log("note: real build info has 0 deps under `go test` — module lookup is covered by the fixture tests, not by this one")
	}
}

func TestCollectReportsRequestedModuleVersion(t *testing.T) {
	collector := newCollector(time.Now, fixtureBuildInfo, noEnv, []string{kueryModule, ismeModule})
	got := collector.Collect()

	if got.Modules[kueryModule] != kueryVersion {
		t.Fatalf("Modules[%s] = %q, want %q", kueryModule, got.Modules[kueryModule], kueryVersion)
	}
	if got.Modules[ismeModule] != ismeVersion {
		t.Fatalf("Modules[%s] = %q, want %q", ismeModule, got.Modules[ismeModule], ismeVersion)
	}
	// Guards the "return the path instead of the version" mutation explicitly:
	// a version must never equal the key it is filed under.
	for modulePath, version := range got.Modules {
		if version == modulePath {
			t.Fatalf("Modules[%s] reported the module path as its version", modulePath)
		}
	}
}

func TestCollectOmitsUnknownModule(t *testing.T) {
	collector := newCollector(time.Now, fixtureBuildInfo, noEnv, []string{kueryModule, absentModule})
	got := collector.Collect()

	if version, ok := got.Modules[absentModule]; ok {
		t.Fatalf("Modules[%s] is present with %q; a module that is not linked in must be ABSENT, not reported as an empty version", absentModule, version)
	}
	if got.Modules[kueryModule] != kueryVersion {
		t.Fatalf("Modules[%s] = %q, want %q", kueryModule, got.Modules[kueryModule], kueryVersion)
	}
}

// TestCollectOmitsModuleReplacedFromDisk covers the local-development build: a
// filesystem replace directive links real code but carries no version at all.
// Reporting it as an empty version would read like a released v0 rather than
// like "this build has no version to give".
func TestCollectOmitsModuleReplacedFromDisk(t *testing.T) {
	replaced := func() (*debug.BuildInfo, bool) {
		buildInfo, _ := fixtureBuildInfo()
		buildInfo.Deps[0] = &debug.Module{
			Path:    kueryModule,
			Version: kueryVersion,
			Replace: &debug.Module{Path: "../kuery", Version: ""},
		}
		return buildInfo, true
	}

	collector := newCollector(time.Now, replaced, noEnv, []string{kueryModule, ismeModule})
	got := collector.Collect()

	if version, ok := got.Modules[kueryModule]; ok {
		t.Fatalf("Modules[%s] = %q for a filesystem replace; a module with no version must be absent", kueryModule, version)
	}
	if got.Modules[ismeModule] != ismeVersion {
		t.Fatalf("Modules[%s] = %q, want %q — unreplaced modules must still resolve", ismeModule, got.Modules[ismeModule], ismeVersion)
	}
}

func TestCollectReportsOnlyCallerSelectedModules(t *testing.T) {
	collector := newCollector(time.Now, fixtureBuildInfo, noEnv, []string{ismeModule})
	got := collector.Collect()

	if len(got.Modules) != 1 {
		t.Fatalf("Modules = %v, want exactly 1 entry — the caller selects which modules are reported, kuery does not dump every dep", got.Modules)
	}
	if got.Modules[ismeModule] != ismeVersion {
		t.Fatalf("Modules[%s] = %q, want %q", ismeModule, got.Modules[ismeModule], ismeVersion)
	}
}

func TestCollectOmitsFlyFieldsWhenEnvUnset(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		collector := newCollector(time.Now, fixtureBuildInfo, noEnv, nil)
		fields := collectJSON(t, collector.Collect())

		for _, key := range []string{"machine", "image"} {
			if raw, ok := fields[key]; ok {
				t.Fatalf("%q is present as %s off Fly; it must be omitted, not blank", key, raw)
			}
		}
	})

	t.Run("set to empty", func(t *testing.T) {
		// Real os.LookupEnv here: an empty-but-set variable reports ok=true, so
		// the guard has to collapse it to "" for omitempty to drop the field.
		t.Setenv(envMachineID, "")
		t.Setenv(envImageRef, "")

		collector := newCollector(time.Now, fixtureBuildInfo, os.LookupEnv, nil)
		fields := collectJSON(t, collector.Collect())

		for _, key := range []string{"machine", "image"} {
			if raw, ok := fields[key]; ok {
				t.Fatalf("%q is present as %s for an empty env var; an empty value is not an answer", key, raw)
			}
		}
	})
}

func TestCollectReportsFlyFieldsWhenEnvSet(t *testing.T) {
	const (
		machineID = "48e561bc1e2f83"
		imageRef  = "registry.fly.io/rainy:deployment-01JZ"
	)
	t.Setenv(envMachineID, machineID)
	t.Setenv(envImageRef, imageRef)

	collector := newCollector(time.Now, fixtureBuildInfo, os.LookupEnv, nil)
	got := collector.Collect()

	if got.Machine != machineID {
		t.Fatalf("Machine = %q, want %q from %s", got.Machine, machineID, envMachineID)
	}
	if got.Image != imageRef {
		t.Fatalf("Image = %q, want %q from %s", got.Image, imageRef, envImageRef)
	}

	fields := collectJSON(t, got)
	for _, key := range []string{"machine", "image"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("%q missing from JSON although its env var is set", key)
		}
	}
}

func TestCollectReportsVCSWhenStamped(t *testing.T) {
	const revision = "6e047fc0d2f1a8b1c3e4f5a6b7c8d9e0f1a2b3c4"

	collector := newCollector(time.Now, fixtureBuildInfoWithVCS(revision, "false"), noEnv, nil)
	got := collector.Collect()

	if got.Revision != revision {
		t.Fatalf("Revision = %q, want %q", got.Revision, revision)
	}
	if got.Modified == nil {
		t.Fatal("Modified is nil although vcs.modified was stamped")
	}
	if *got.Modified {
		t.Fatal("Modified = true, want false for vcs.modified=false")
	}

	// A clean tree is a meaningful false and must survive omitempty — that is
	// why Modified is a pointer.
	fields := collectJSON(t, got)
	if raw, ok := fields["modified"]; !ok || string(raw) != "false" {
		t.Fatalf("modified in JSON = %s (present=%v), want false", raw, ok)
	}
}

func TestCollectOmitsVCSWhenNotStamped(t *testing.T) {
	// The deployed case: .dockerignore excludes .git, so nothing is stamped.
	collector := newCollector(time.Now, fixtureBuildInfo, noEnv, nil)
	fields := collectJSON(t, collector.Collect())

	for _, key := range []string{"revision", "modified"} {
		if raw, ok := fields[key]; ok {
			t.Fatalf("%q is present as %s for a build with no vcs settings; it must be omitted", key, raw)
		}
	}
}

func TestUptimeGrowsWhileStartedAtIsStable(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)}
	start := clock.Now()

	collector := newCollector(clock.Now, fixtureBuildInfo, noEnv, nil)
	first := collector.Collect()

	clock.Advance(7 * time.Second)
	second := collector.Collect()

	if first.UptimeSeconds != 0 {
		t.Fatalf("first UptimeSeconds = %d, want 0", first.UptimeSeconds)
	}
	if second.UptimeSeconds != 7 {
		t.Fatalf("second UptimeSeconds = %d, want 7 — uptime must grow with the clock", second.UptimeSeconds)
	}
	if !second.StartedAt.Equal(first.StartedAt) {
		t.Fatalf("StartedAt changed between calls: %s then %s — it is the process start, not the call time", first.StartedAt, second.StartedAt)
	}
	if !first.StartedAt.Equal(start) {
		t.Fatalf("StartedAt = %s, want the construction time %s", first.StartedAt, start)
	}
}

// TestNewTakesNoDatabaseOrContext is the compile-level guard for the rule in
// the package doc: this endpoint must never touch a database, an external
// service or a lock. That cannot be asserted at runtime, so instead the
// exported shape is pinned — a *bun.DB, a *sql.DB, a connection or a
// context.Context could only be threaded in by changing one of these
// signatures, and changing one fails here.
func TestNewTakesNoDatabaseOrContext(t *testing.T) {
	newType := reflect.TypeOf(New)
	if !newType.IsVariadic() || newType.NumIn() != 1 {
		t.Fatalf("New has %d parameter(s) (variadic=%v); it must take exactly one variadic module-path list and nothing else", newType.NumIn(), newType.IsVariadic())
	}
	if got, want := newType.In(0), reflect.TypeOf([]string(nil)); got != want {
		t.Fatalf("New parameter type = %s, want %s — a non-string parameter means a dependency (db, conn, ctx) was threaded in", got, want)
	}
	if newType.NumOut() != 1 || newType.Out(0) != reflect.TypeOf((*Collector)(nil)) {
		t.Fatalf("New must return exactly *Collector, got %v outputs", newType.NumOut())
	}

	collectMethod, ok := reflect.TypeOf((*Collector)(nil)).MethodByName("Collect")
	if !ok {
		t.Fatal("Collector has no exported Collect method")
	}
	// NumIn()==1 is the receiver alone: no ctx, no options, no handle.
	if collectMethod.Type.NumIn() != 1 {
		t.Fatalf("Collect takes %d argument(s) besides the receiver; it must take none (a context.Context here is how a DB call arrives)", collectMethod.Type.NumIn()-1)
	}

	// The struct must hold no interface/pointer field that could carry I/O.
	// Its only non-seam state is the module list and the start time.
	collectorType := reflect.TypeOf(Collector{})
	allowed := map[string]bool{"modulePaths": true, "startedAt": true, "now": true, "readBuildInfo": true, "lookupEnv": true}
	for i := range collectorType.NumField() {
		field := collectorType.Field(i)
		if !allowed[field.Name] {
			t.Fatalf("Collector gained field %q (%s): every field must be a module list, a timestamp, or one of the three pure seams — anything else is a dependency this endpoint must not have", field.Name, field.Type)
		}
	}
}
