---
name: postgres-sslmode-default-is-require
description: bun/db defaults an empty Config.SSLMode to "require", not "disable" — a non-TLS server now needs SSLMode:"disable" spelled out
metadata:
  node_type: memory
  type: project
---

`bun/db.Config.SSLMode` left empty produces `sslmode=require`. It used to produce
`sslmode=disable`.

**Why:** a shared library cannot tell a loopback socket from a managed Postgres
across the public internet, and it is handed only a hostname. Defaulting to
`disable` sends credentials and row data in cleartext and tells nobody; the
wrong guess in the `require` direction produces a startup error naming sslmode,
which an operator can answer. Failing closed is the only default that cannot
leak silently.

**How to apply:** connecting to a local Postgres without TLS through the
discrete `Host/Port/User/Password/DBName` fields now needs `SSLMode: "disable"`
written out. That is the intent — cleartext becomes a decision on the record.

⚠️ It reached **no** existing consumer, and the reason is not the one that gets
assumed. It is *not* only that `PostgresDSN` takes precedence (it does, and
gardener/isme/rainy all pass one). It is also that all three pass `SSLMode`
through from their own config, where `envconfig:"DB_SSLMODE" default:"disable"`
already fills it in — so the empty-string branch was never taken by a service
either way. Check both before assuming a consumer is exposed to a change in this
default.

Part of [[kuery-security-fixes-2026-09]].
