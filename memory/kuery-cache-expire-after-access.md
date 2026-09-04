---
name: kuery-cache-expire-after-access
description: "kuery/cache TTL is expire-after-ACCESS (otter), not expire-after-write — a hot key never expires, so it is unsafe for staleness-bounded caches"
metadata: 
  node_type: memory
  type: reference
  modified: 2026-08-11T08:42:35.974Z
---

`kuery/cache` (`cache.go`, backed by `maypok86/otter/v2`) builds its cache with **`otter.ExpiryAccessingFunc`**. Per `otter/v2@v2.2.1/expiry_calculator.go:124-126`, that means an entry is deleted once the duration elapses after creation, value replacement, **or its last READ**.

So `Set(key, v, 60*time.Second)` does NOT mean "this value is at most 60s stale". A key read more often than once per 60s **refreshes its own TTL on every read and never expires**. The package doc comment ("expires after that duration") reads like expire-after-write and hides this.

Harmless for a pure memoization cache (hot keys stay hot, that is the point). **Wrong and silently dangerous** whenever the TTL is doing correctness work — bounding staleness against an authoritative store. Canonical trap: a read-through auth/session cache where a busy session refreshes its own entry forever and never re-reads the DB, so a revocation is never observed. The failure only shows under sustained traffic, which is exactly when it matters.

**Why:** otter offers `ExpiryAccessing` / `ExpiryWriting` / `ExpiryCreating`; kuery picked accessing and exposes no way to choose.
**How to apply:** if the TTL bounds staleness, do not trust it — store a `checkedAt time.Time` inside the cached value and compare `time.Since(checkedAt)` in your own code, treating otter's TTL as memory reclamation only. Used this way in gardener's revocation store ([[gardener-session-revocation]]). Alternatively add an expire-after-write variant to kuery/cache (not done as of 2026-08-11).
