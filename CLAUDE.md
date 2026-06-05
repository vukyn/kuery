# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
| `middleware/gin/rate_limiter` | Gin + tollbooth | only Gin-specific package |
| `log` | zerolog | `log.Logger` (chainable WithField/WithPkg/WithFunc) and **`log.SimpleLogger`** — the interface other packages (`graceful`, `bun/hooks`) accept for logging |
| `graceful` | — | signal-driven shutdown: `GracefulShutdown(handlers, opts)`, `ShutdownWithCallback`, `SimpleShutdown`; logs via `log.SimpleLogger` |

Cross-package dependency chains to keep in mind when editing: `jwt → claims → cryp`, `ctx → claims`, `bun/query → http/base`, `bun/hooks`/`graceful` → `log`. `http/errors → http/base` (for `Forward`).

## Conventions

Shared with the service repos (see `../CLAUDE.md`): `any` not `interface{}`, no abbreviated variable names, snake_case filenames, import groups stdlib | third-party | intra-module. Packages here must stay application-agnostic — no knowledge of any specific service's domains or config.
