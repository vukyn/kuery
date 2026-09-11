# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## The memory layer

@MEMORY.md

⚠️ **That import is the point of the file, not decoration.** `MEMORY.md` and
`memory/` are the distilled layer — one hard-won fact per file, with why it
matters — and they live **in the repository** because a machine's own Claude
memory directory is workspace-scoped and machine-local: this repo opened on
another machine, or outside the workspace the notes were written in, arrived with
none of them.

It is a **distillation, not the record.** This file and the repository's other
documents stay the authority; where a note disagrees with the file that owns the
subject, the repository wins and the note is what to fix. `MEMORY.md` carries the
rules the notes are written under — one line per note in the index, one fact per
file, say why rather than only what, and delete a wrong note rather than adding a
second one beside it.

## What this repo is

`github.com/vukyn/kuery` — the shared utility library for the pet-platform services (siblings of this repo under `../`; the set of services grows over time — don't assume a fixed list). Any code reusable across those services belongs here as a package, gets a version tag, and is imported via go.mod — never duplicated inside a service. Services have no local `pkg/` directories anymore (consolidated here in v1.12.0).

Only the **v1 module line** is managed — module path is `github.com/vukyn/kuery`, never `/v2`. Versioning is informal: breaking changes have shipped in minor bumps (e.g. v1.12.0 removed `graceful.ShutDownSlowly`), so when bumping consumers, build them before tagging.

## Commands

```bash
go build ./...                # compile everything (no main package)
make lint                     # go mod tidy + golangci-lint + govulncheck
go test ./...                 # no _test.go files exist yet (test-math Makefile target is stale — no math/ pkg)

# Release flow
make v-tag                    # list tags, newest first
make v-tag-latest             # newest tag only
make tag VERSION=1.13.0       # creates annotated tag v1.13.0 AND pushes it
```

**Tag retention rule: keep only the 5 newest version tags.** After tagging a release, delete older tags both locally (`git tag -d`) and on the remote (`git push origin --delete refs/tags/<tag>`). Old versions stay fetchable for consumers via the proxy.golang.org cache.

After a release: `go get github.com/vukyn/kuery@v<new>` in each consuming service.

## Architecture

Flat top-level packages, no `internal/`, no main. Two tiers:

**Pure-Go utilities** (stdlib or minor deps only): `conv` (type conversion, `ToPointer`, `IsZero`), `cryp` (UID/ULID, `aes/`, `rand/`), `file` (+ `macos/`, `windows/`), `query` (generic slice helpers: Index/Find/Where/Map…), `t` (generic numeric type constraints, used by other packages), `validator` (govalidator wrapper), `network`, `monitor`, `simplelog` (fmt-based leveled printing).

**Framework-coupled packages** — these carry the heavyweight deps in go.mod:

| Package | Framework | Notes |
|---|---|---|
| `http/{base,errors,fiber}` | Fiber v2 | `base.Pagination`/`Response` DTOs; `errors` constructors implementing `errors.Error` (message+status); `fiber.OK/Err` response funnel — `Err` maps `errors.Error` status, special-cases 401 |
| `ctx` | Fiber + sarulabs/di | Fiber-locals ↔ `context.Context` bridge (user_id/email/token_id/is_admin/client_ip/user_agent) + request-scoped DI container accessors |
| `recover` | Fiber | panic-recovery middleware (`NewFiberRecover`) |
| `claims`, `jwt` | golang-jwt/v5 | `jwt` → `claims` → `cryp` (ULID for jti). HS256 + RS256 generate/validate |
| `bun/{hooks,query}` | uptrace/bun | query-logging hook; `SelectWithPagination` takes `http/base.Pagination` (cross-tier import) |
| `http/health` | **none** (stdlib only) | deployment verification: `New(modulePaths...)` → `Collect()` reports the module versions the running binary was BUILT with, read from `debug.ReadBuildInfo()` — no ldflags, no `-X`, no `.git` needed. Fiber wrappers live in `http/fiber` + `http/fiberv3` so a non-Fiber caller imports no framework. ⚠️ **Does no I/O by design** — no DB, no external call, no lock: consumers run scale-to-zero over a suspending Postgres, so a DB-pinging health check would wake it on every call, and "process up" ≠ "database reachable". `revision`/`modified` appear only when VCS-stamped (absent in Docker builds that `.dockerignore` `.git`); `machine`/`image` come from `FLY_MACHINE_ID`/`FLY_IMAGE_REF`. Live at rainy `/api/v1/__version` |
| `http/ratelimit` | Fiber v2 | fixed-window in-memory limiter with a **`CountStatus` predicate** — default `CountAuthFailures` charges 401/403 only, so a 5xx outage or a malformed-body 400 never spends budget (Fiber's limiter charges every `>= 400`). Neither predicate counts 429, so chained tiers can't charge each other. Warns once per key per window; `MaxKeys`-bounded, and an entry refunded back to zero is **deleted** so an uncounted request leaves no key behind (residue would let an attacker-chosen keyspace fill the map at zero budget cost, and a full map evicts real callers' buckets); `OnBlocked` hook for releasing per-request resources on the short-circuit path. **Clones the key** — Fiber strings alias fasthttp's buffer |
| `middleware/gin/rate_limiter` | Gin + tollbooth | only Gin-specific package; **`http/ratelimit` is the Fiber equivalent**, not a port of this |
| `log` | zerolog | `log.Logger` (chainable WithField/WithPkg/WithFunc) and **`log.SimpleLogger`** — the interface other packages (`graceful`, `bun/hooks`) accept for logging |
| `graceful` | — | signal-driven shutdown: `GracefulShutdown(handlers, opts)`, `ShutdownWithCallback`, `SimpleShutdown`; logs via `log.SimpleLogger` |

Cross-package dependency chains to keep in mind when editing: `jwt → claims → cryp`, `ctx → claims`, `bun/query → http/base`, `bun/hooks`/`graceful` → `log`. `http/errors → http/base` (for `Forward`), `http/ratelimit → http/{errors,fiber}` + `log` + `text`.

⚠️ **Fiber v2 strings are not yours to keep.** `c.Get(...)`, `c.Body()`-derived values and `c.IP()` on the `ProxyHeader` path are zero-copy views into fasthttp's request buffer (`app.getString` = `utils.UnsafeString` unless `Config.Immutable`), so the next request overwrites them. Any Fiber-coupled package that RETAINS such a string past the response — a map key, a cached value, a queued item — must `strings.Clone` it first. `http/ratelimit` does; Fiber's own limiter survives only because its storage layer copies defensively.

**kuery clones at the two egress points where a Fiber-owned string would otherwise leave the request:** `ctx.GetUserAgentFromFiberCtx` and `ctx.GetClientIPFromFiberCtx` (and their `ctxv3` twins). Every service calls these per request through `NewContextFromFiberCtx`, so cloning here makes consumers safe by default rather than by accident — a service that stashes the client IP in a cache or a goroutine no longer has a latent bug. `ctx/context_test.go` compares backing-array pointers (the values are equal either way, which is what makes the bug invisible) and fails if either clone is removed. Do not "simplify" them away.

## Conventions

Shared with the service repos (see `../CLAUDE.md`): `any` not `interface{}`, no abbreviated variable names, snake_case filenames, import groups stdlib | third-party | intra-module. Packages here must stay application-agnostic — no knowledge of any specific service's domains or config.
