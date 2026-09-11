# Memory Index

## Moved in from the platform-level store (2026-09-05)

⚠️ These were written while the session's working directory was the platform
root, so they landed in `<root>/.claude/agent-memory/` where this repo's own
agent could not see them — same knowledge, same agent, different cwd. They live
here now, and new ones belong here.

- [kuery consolidation](project_kuery_consolidation.md) — all repos consolidated on kuery, no local pkg/; medioa2+isme now v1.15.0; gotcha: GetUserId → GetUserID
- [debug.ReadBuildInfo is empty under go test](go-buildinfo-empty-under-test.md) — ok=true but Deps empty and no vcs.*; version tests need a synthetic fixture or they pass on a no-op impl
