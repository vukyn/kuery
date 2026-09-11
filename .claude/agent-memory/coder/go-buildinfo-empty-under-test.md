---
name: go-buildinfo-empty-under-test
description: debug.ReadBuildInfo() under `go test` returns ok=true but Deps is EMPTY and no vcs.* settings — version-reporting tests must use a synthetic BuildInfo fixture, not the runtime
metadata:
  type: feedback
---

Any test that asserts on module versions from `debug.ReadBuildInfo()` must feed a
**synthetic `*debug.BuildInfo` fixture** through an injected seam, plus a guard test
that the fixture is non-empty. Asserting against the runtime's build info passes
vacuously.

**Why:** measured in kuery 2026-09-11 (go1.27.1, `go test ./http/health/`):
`ok=true`, `GoVersion="go1.27.1"`, **`len(Deps)==0`** (a test binary for a
stdlib-only package links nothing) and **zero `vcs.*` settings** (test builds are
not VCS-stamped even in a git worktree). So a lookup that always returns nothing
passes every "module version" assertion. Same trap class as
[[fixture-hidden-branch]]: the test must be able to fail.

Related facts verified against a real compiled binary (`go version -m`):
- A **directory `replace`** reports `Replace.Version == "(devel)"`, not `""` — a
  local dev build is reported as `(devel)`, which is the honest answer. Some
  replace forms do carry an empty version; guard for it.
- `vcs.revision` / `vcs.modified` are absent from any image whose build context
  excludes `.git` (every service here — see each `.dockerignore`). Deployed
  builds identify themselves by **module versions**, never by revision.

**How to apply:** when building or changing anything that reads build info,
inject `readBuildInfo func() (*debug.BuildInfo, bool)` as a test seam, and make
the one test that touches the real runtime assert only `GoVersion` — failing
loudly (`t.Fatal`) if `ok==false` rather than skipping.
