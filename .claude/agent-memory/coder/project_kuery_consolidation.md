---
name: kuery-consolidation-migration
description: Local pkg/ consolidated into kuery; all 4 repos done (memz migrated 2026-06-06); all on kuery v1.13.0; gotcha GetUserId renamed GetUserID
metadata:
  type: project
---

The local `pkg/` utilities (claims, ctx, jwt, graceful, recover, bun/hooks, bun/query, http/base, http/errors, http/fiber) were consolidated into `github.com/vukyn/kuery` v1.12.0. All four repos migrated: isme, rainy, medioa2 (2026-06-06 kuery bump) and memz (2026-06-06 gin→Fiber migration, which also deleted memz's legacy `pkg/{recover,xhttp,env}` and moved it onto kuery http/fiber + http/errors + recover). memz still differs structurally from the other services (mongo, service/ layer between usecase and repository, init/ wiring instead of sarulabs/di).

All four consumer repos were bumped to kuery v1.13.0 (security fixes, go directive 1.26.4) on 2026-06-06. memz jumped from v1.5.0 and needed source migration: kuery deleted `routine.Run` and `conv.ReadInterface` (no replacements exist in kuery — memz got private local helpers), `log.New(pkg, fn)` became `log.New().WithPkg().WithFunc()` with `Errorf` instead of `Error(msg, err)`, and `graceful.ShutDownSlowly` became `ShutdownWithCallback`.

**Why:** eliminates the copy-pasted pkg/ template shared across the three services.

**How to apply:** path mapping: `<mod>/pkg/base` → `kuery/http/base`, `<mod>/pkg/{claims,ctx,graceful,recover,jwt}` → `kuery/<same>`, `<mod>/pkg/bun/{hooks,query}` → `kuery/bun/<same>`, `<mod>/pkg/http/{errors,fiber}` → `kuery/http/<same>`. Import aliases (pkgCtx, pkgClaims, etc.) carry over unchanged but the kuery imports must move from the internal group to the third-party group. Gotchas: kuery/ctx renamed `GetUserId` → `GetUserID` (grep lowercase-`Id` calls; rainy was unaffected); v1.12.0 graceful kept ShutdownWithCallback/DefaultSignals/ShutdownOptions; http/errors kept InvalidRequest/DatabaseError/NotFound/InternalServerError and added Unauthorized/Forbidden/Forward. go.mod files have a commented `// replace ... => ../kuery` line — keep it commented. Each service's CLAUDE.md "Shared packages (pkg/)" section goes stale after migration — flag it.
