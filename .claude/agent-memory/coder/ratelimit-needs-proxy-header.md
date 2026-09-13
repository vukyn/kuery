---
name: ratelimit-needs-proxy-header
description: Mounting kuery/http/ratelimit in a platform service is incomplete without fiber.Config.ProxyHeader + EnableIPValidation — behind fly-proxy an IP-keyed tier is ONE GLOBAL bucket, which is worse than no limiter
metadata:
  type: feedback
---

Before mounting `kuery/http/ratelimit` (or anything else keyed on `c.IP()`) in a
platform service, check whether that service sets `fiber.Config.ProxyHeader`.
Most do not. Setting the header is part of the limiter change, not a follow-up.

**Why:** every fly.io service sits behind fly-proxy. With `ProxyHeader` unset,
`c.IP()` returns the socket's remote address — the proxy's — so every caller keys
the same bucket. The limiter then does not throttle an attacker, it throttles
*everyone at once* the moment one attacker spends the budget: strictly worse than
no limiter. `EnableIPValidation: true` must go in with it, or Fiber returns the
header verbatim with no socket fallback, and then an ABSENT header yields an empty
key (one global bucket again) while a MALFORMED one becomes a fresh key, so
rotating junk through the header mints unlimited budgets. gardener documents both
halves; rainy had neither until the limiters landed (2026-09-11), and the fix was
`APP_PROXY_HEADER` config → `fiber.Config` → `fly.toml [env] = 'Fly-Client-IP'`.

**How to apply:** the checklist for a new inbound limiter in any of the six
clean-arch services is (1) the tier + its `CountStatus` predicate, (2)
`ProxyHeader` + `EnableIPValidation` in `fiber.Config` from an `APP_PROXY_HEADER`
env var, (3) that var set in `fly.toml`, (4) a boot warn when it is empty in
production — the mis-binding is otherwise silent and only shows up as a mass
lockout. Also verify per-caller keying in a test (mount with a test proxy header,
spend one address's budget, assert a second address is unaffected); a global
bucket passes every single-caller test. As of 2026-09-11 **isme, medioa2 and memz
still have no inbound limiter and no proxy header** — see [[di-container-leak-per-service]]
for the same "fixed in one service, live in the others" pattern.
