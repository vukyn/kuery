---
name: kuery-security-fixes-2026-07
description: kuery security scan 2026-07-15 → fixed otel DoS + Go toolchain 1.26.5 (reachable crypto/tls ECH) + file perms 0600 + aes/migrate tests; Fiber CVE deferred to v3 twins
metadata: 
  node_type: memory
  type: project
---

kuery security scan 2026-07-15 (18 findings: 0 crit, 1 high, 5 med, 12 low; gitleaks clean). Fixes applied (uncommitted batch, then + [[kuery-fiber-v3-twins]] on top):

- **otel DoS CVE-2026-29181 (High):** `go.opentelemetry.io/otel` v1.40→v1.41.0.
- **crypto/tls ECH leak CVE-2026-42505 (Med, govulncheck REACHABLE):** added `toolchain go1.26.5` to go.mod (also clears os symlink-escape CVE-2026-4970). Left `go 1.26.4` directive as-is.
- **file/file.go perms G302/G306:** 0644→0600 at 4 write/open sites (owner-only).
- **new tests:** `cryp/aes/aes_test.go` (Encrypt/Decrypt round-trip + deriveKey), `bun/migrate/migrate_test.go` (Run/RollbackLast on temp sqlite via sqliteshim).

Deferred by design: Fiber X-Real-IP spoof CVE-2026-45045 → handled via v3 twins not in-place bump ([[kuery-fiber-v3-twins]]). Documented-by-design / false-positive (NO change): MD5 checksum (cryp/uid.go, 0 callers), file-util path traversal G304/G703, X-API-Key header-name G101, int-overflow G115, unreachable openpgp GO-2026-5932.

SHIPPED 2026-07-15: PR#31 (squash e03d400) → **tagged v1.41.0** (pruned v1.36.0). Scan log: `.claude/security-scans/kuery/1784094744.json`. Follows [[kuery-shared-lib-rule]]. Consumers must `go get github.com/vukyn/kuery@v1.41.0` to actually receive the CVE fixes.
