---
name: kuery-security-fixes-2026-09
description: kuery security scan 2026-09-11 → bun hook logged inlined SQL values, Postgres DSN built with Sprintf + defaulted to sslmode=disable, unbounded argon2 memory, silent empty random token; all fixed, x/text forced past the requested version
metadata:
  node_type: memory
  type: project
---

kuery security fixes on top of v1.59.0 (uncommitted batch). Five findings, all
fixed; the parts worth keeping are the ones a diff does not explain.

**The query hook was the headline and was overstated in the report.** `bun/hooks`
logged `event.Query`, which bun fills with the SQL **values already inlined** —
argon2id hashes, refresh tokens, session ids, OAuth codes, API keys. The fix is
one field: `event.QueryTemplate` keeps the placeholders (`QueryArgs` holds the
values and is not logged). But **production was never leaking**: the call is
`Infof` and isme/medioa2/rainy/gardener all set `LOGGER_LEVEL='warn'` in
`fly.toml`, so Info was filtered. The leak was on developer machines (isme,
medioa2, gardener `.env` = debug; rainy = info). The reason it still had to
change is that one config value was the whole defence — and memz and tomatime set
no `LOGGER_LEVEL` in `fly.toml` at all. There is **no second copy of this hook**:
the gobuild `platform-service` template only *calls* `NewQueryHook`, so fixing
kuery fixed generated services too.

**`golang.org/x/text` could not be pinned to the requested v0.39.0.**
`x/crypto@v0.56.0` and `x/image@v0.45.0` both `require x/text v0.41.0`, so MVS
rejects the lower pin outright — the whole `go get` fails, it does not partially
apply. v0.41.0 clears GO-2026-5970 (the `unicode/norm` infinite loop, reachable
through `text/fold.go` `FoldVN` and therefore through rainy's search input) and
more. Expect the same whenever a scan report names a floor lower than a sibling
x/ module already demands.

**`GO-2026-5932` (x/crypto/openpgp unmaintained) still reports and has no fixed
version** — same as the 2026-07 scan. Not called by kuery. Leave it.

Also fixed: the Postgres DSN was assembled with `fmt.Sprintf`, so a password
containing `@ / ? #` escaped its field and could move the host or override
sslmode (now `net/url`); an empty `Config.SSLMode` defaulted to `disable` (now
`require` — see [[postgres-sslmode-default-is-require]]); `CompareArgon2id` took
`memory` unbounded from the stored hash; `RandMixedString` returned `""` on an
unreachable error branch instead of panicking.

Follows [[kuery-shared-lib-rule]] — consumers must `go get` the new tag to
receive any of it.
