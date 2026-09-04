---
name: kuery-fiber-v3-twins
description: kuery Fiber v2→v3 as side-by-side v3-suffixed twin packages (fiberv3/ctxv3/recoverv3/rbacv3/authv3) for gradual consumer migration; v2 untouched; fixes CVE-2026-45045
metadata: 
  node_type: memory
  type: project
  modified: 2026-08-11T08:42:19.387Z
---

kuery Fiber v3 migration = **side-by-side twins, NOT in-place bump**. Fiber v2 (`gofiber/fiber/v2`) and v3 (`gofiber/fiber/v3` v3.4.0) coexist in one go.mod (different module paths). 5 fiber-coupled pkgs got v3-suffixed twins, v2 originals LEFT INTACT: `http/fiberv3`, `ctxv3`, `recoverv3`, `rbacv3`, `authv3`. Consumer migration (isme/medioa2/rainy) = **pure import-path swap**, done gradually one service at a time.

The v2→v3 transform is mechanical: `*fiber.Ctx` → `fiber.Ctx` (v3 Ctx is a value interface, drop pointer). Everything else carries over (`fiber.Handler`, `fiber.Map`, `fiber.StatusX`, `fiber.HeaderAuthorization`, `c.Locals/Next/Get/Status/Method/OriginalURL/IP`). kuery's own string Locals keys unaffected — v3's context-key change only hit *middleware-provided* values. Fiber v3 needs Go 1.25+ (kuery on 1.26 ✓). authv3/rbacv3 import ctxv3+fiberv3 twins, never v2.

**Why:** CVE-2026-45045 (X-Real-IP spoof via Header.Add) fixed in fiber v3; kuery is shared so in-place bump would force all consumers to migrate at once. Twins decouple the timeline.
**How to apply:** when a consumer moves to fiber v3, swap `kuery/http/fiber`→`kuery/http/fiberv3`, `kuery/ctx`→`kuery/ctxv3`, etc. Once ALL consumers migrated, delete the v2 twins + drop fiber/v2 from go.mod.

SHIPPED 2026-07-15: PR#31 (squash e03d400) → **tagged v1.41.0** alongside kuery security fixes ([[kuery-security-fixes-2026-07]]). Follows [[kuery-shared-lib-rule]] (pruned v1.36.0, keep 5 newest). Consumers NOT yet `go get`'d — each service migrates fiber v3 gradually on its own timeline. CLAUDE.md undercounts fiber-coupled pkgs (missed rbac+auth).

⚠️ **TWINS NOW DRIFT — v2-only features land in v2 twins and silently vanish on migration.** 2026-08-11: session revocation ([[gardener-session-revocation]]) added `RevocationChecker` + `NewAuthMiddlewareWithRevocation` to `auth/` and session-id set/get to `ctx/` — **`authv3`/`ctxv3` did NOT get them** (decided: defer, no v3 consumer needs it). `claims` is fiber-free so `sid` IS on both lines. A service swapping `kuery/auth`→`kuery/authv3` loses its revocation check **with no compile error** (the v3 ctor simply has no checker param). Also `ctxv3` has **zero test files**, so none of the v2 coverage carries over.
**How to apply:** before ANY consumer migrates to a v3 twin, diff the twin against its v2 original for missing features — do not assume the twins are still byte-parallel. Adding to a v2 twin? Either port to v3 in the same PR or record the gap here.
