// Package ratelimit is a fixed-window, in-memory Fiber v2 rate limiter that
// charges a caller only for the responses a policy says should count.
//
// # Why not fiber/middleware/limiter
//
// Fiber's own limiter can only skip "successful" (status < 400) or "failed"
// (status >= 400) requests, which lumps together answers that mean very
// different things:
//
//   - A 500 spends budget. During a database outage every login answers 500, so
//     after Max attempts the limiter ADDS a lockout on top of the outage — one
//     that outlives the recovery.
//   - A 400 spends budget. A client posting a malformed body locks its own
//     address out without ever guessing a password.
//
// Neither is fixable from outside Fiber's limiter: its Next hook runs before the
// handler, so it cannot see the status, and LimitReached only fires once a caller
// is already blocked, so there is no refund hook.
//
// This package takes a CountStatus predicate instead. The default,
// CountAuthFailures, charges only 401 and 403: a rejected credential is the only
// thing a login limiter should charge for. 5xx is the server's fault, 400 is a
// malformed request, and 408 is a timeout.
//
// # 429 is never counted
//
// Both shipped predicates ignore 429, which removes an ordering trap that Fiber's
// limiter has. Limiters are usually CHAINED — a tight tier keyed on
// address+username in front of a loose tier keyed on the address alone, because
// one key cannot stop both "guessing one account" and "spraying many accounts".
// With Fiber, a 429 is a 4xx, so a single user blocked by the tight tier keeps
// charging 429s to the loose tier's shared-address budget and can lock out
// everyone behind that address (a heavily CGNAT'd mobile network puts a whole
// crew of users on one public address). Here an inner tier's block never charges
// an outer tier, so the chain stays correct whichever way it is ordered.
//
// # Counting happens before the handler
//
// A slot is reserved BEFORE c.Next() and refunded afterwards when CountStatus
// says the status should not count. Counting only after the response would let
// many concurrent requests through a slow (database-bound) handler before the
// counter ever moved.
//
// # Logging
//
// A blocked flood writes ONE warn line per key per window, not one per request:
// an attacker who is already being refused must not be able to keep growing the
// logs. Only the Scope label, the key, Max and Window are logged, and the key is
// passed through text.SanitizeUntrusted because it is attacker-supplied.
//
// # Bounded memory
//
// The key is attacker-influenced, so the map is bounded (MaxKeys) and expired
// entries are swept lazily. When the map is full after a sweep, the entry closest
// to expiry is evicted — the limiter fails OPEN, because a full map must never
// lock out real users.
//
// # Per-process counters
//
// The store is in-memory, so counters are per process: behind N machines a caller
// gets Max attempts per machine. Fine for a single-machine deployment; a shared
// store is the fix if that changes.
package ratelimit

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	pkgErr "github.com/vukyn/kuery/http/errors"
	pkgHttp "github.com/vukyn/kuery/http/fiber"
	pkgLog "github.com/vukyn/kuery/log"
	pkgText "github.com/vukyn/kuery/text"
)

// Defaults applied when the matching Config field is left zero.
const (
	// DefaultWindow is the fallback window length. A misconfigured (non-positive)
	// window must not mean "no window".
	DefaultWindow = 5 * time.Minute

	// DefaultMaxKeys bounds how many distinct keys are tracked at once.
	DefaultMaxKeys = 10_000

	// DefaultMessage is the generic 429 body message. It is deliberately English
	// and product-neutral: a shared package must not carry one service's
	// user-facing copy — pass Message for localized wording.
	DefaultMessage = "Too many attempts. Please wait a few minutes and try again."
)

// Rate-limit response headers, matching fiber/middleware/limiter so a client that
// already understands Fiber's limiter needs no changes.
const (
	HeaderLimit     = "X-RateLimit-Limit"
	HeaderRemaining = "X-RateLimit-Remaining"
	HeaderReset     = "X-RateLimit-Reset"
)

// keyLogLimit bounds how much of the (attacker-supplied) key reaches the log.
const keyLogLimit = 128

// Config describes one rate-limit tier.
type Config struct {
	// Max is how many counted responses one key may produce per Window before it
	// is blocked. Zero or less disables the middleware entirely (pass-through) —
	// a limiter with a zero budget would block everyone.
	Max int

	// Window is the fixed window length. Zero or less falls back to DefaultWindow.
	Window time.Duration

	// KeyFunc extracts the bucket key from the request — the client address, or
	// the address plus the attempted username for a tighter tier. Required: a nil
	// KeyFunc is a pass-through with a warn, because putting every caller in one
	// bucket would lock the whole service out.
	KeyFunc func(*fiber.Ctx) string

	// CountStatus decides whether a response status spends budget. Nil means
	// CountAuthFailures.
	CountStatus func(status int) bool

	// Message is the 429 body message. Empty means DefaultMessage.
	Message string

	// Scope labels the tier in the log line (e.g. "ip", "ip+username"). It has no
	// effect on behaviour.
	Scope string

	// OnBlocked runs just BEFORE the 429 is written. A short-circuiting
	// middleware never reaches the handler, so anything the handler would have
	// released has to be released here — gardener uses it to delete the
	// request-scoped DI sub-container that the handler would otherwise have
	// deleted. Optional.
	OnBlocked func(*fiber.Ctx)

	// MaxKeys bounds the tracked key set. Zero or less means DefaultMaxKeys.
	MaxKeys int

	// Clock is the time source, injectable so a test can advance a window instead
	// of sleeping through it. Nil means time.Now.
	//
	// It is exported on purpose: a consuming service mounts its own tiers, and
	// without a seam its only way to test that a lockout actually expires is to
	// sleep the real window — which means either a slow suite or an artificially
	// short window that tests nothing like production.
	Clock func() time.Time
}

// New builds the middleware. It returns a pass-through handler when the tier is
// disabled (Max <= 0) or misconfigured (nil KeyFunc).
func New(cfg Config) fiber.Handler {
	handler, _ := newMiddleware(cfg)
	return handler
}

// middleware is the resolved configuration plus its key store.
type middleware struct {
	keys        *store
	keyFunc     func(*fiber.Ctx) string
	countStatus func(status int) bool
	onBlocked   func(*fiber.Ctx)
	message     string
	scope       string
	max         int
	window      time.Duration
}

// newMiddleware also returns the store so tests can inspect the bookkeeping that
// is not observable from a response. The store is nil for a pass-through.
func newMiddleware(cfg Config) (fiber.Handler, *store) {
	if cfg.Max <= 0 {
		// Explicitly disabled.
		return passthrough, nil
	}
	if cfg.KeyFunc == nil {
		// Misconfiguration: fail open, but say so loudly.
		pkgLog.New().WithPkg("ratelimit").Warnf("disabled: KeyFunc is nil (scope=%q)", cfg.Scope)
		return passthrough, nil
	}

	window := cfg.Window
	if window <= 0 {
		window = DefaultWindow
	}
	countStatus := cfg.CountStatus
	if countStatus == nil {
		countStatus = CountAuthFailures
	}
	message := cfg.Message
	if message == "" {
		message = DefaultMessage
	}
	maxKeys := cfg.MaxKeys
	if maxKeys <= 0 {
		maxKeys = DefaultMaxKeys
	}
	now := cfg.Clock
	if now == nil {
		now = time.Now
	}

	limiter := &middleware{
		keys:        newStore(cfg.Max, window, maxKeys, now),
		keyFunc:     cfg.KeyFunc,
		countStatus: countStatus,
		onBlocked:   cfg.OnBlocked,
		message:     message,
		scope:       cfg.Scope,
		max:         cfg.Max,
		window:      window,
	}
	return limiter.handle, limiter.keys
}

func passthrough(c *fiber.Ctx) error {
	return c.Next()
}

func (m *middleware) handle(c *fiber.Ctx) error {
	// strings.Clone is load-bearing, not defensive style. Fiber v2 hands out
	// zero-copy strings that point into fasthttp's request buffer (c.Get, a
	// parsed body field — anything that is not a fresh concatenation), and that
	// buffer is recycled after the response. Keeping such a string as a map key
	// lets the key MUTATE while it is in the map: entries become unfindable (so
	// the caller silently gets a fresh budget every request), the map grows
	// without bound, and a mutated key can collide with another caller's bucket.
	// KeyFunc is caller-supplied, so the copy has to happen here.
	key := strings.Clone(m.keyFunc(c))

	reserved := m.keys.reserve(key)
	if !reserved.allowed {
		return m.block(c, key, reserved)
	}

	err := c.Next()

	// Refund the slot when the answer is not the caller's fault.
	if !m.countStatus(responseStatus(c, err)) {
		m.keys.refund(key)
	}
	return err
}

// block short-circuits the chain: headers, the caller's cleanup hook, one warn
// line per key per window, then the standard {code, message} 429 envelope.
//
// The X-RateLimit-* headers are set here only, not on allowed responses (where
// Fiber's limiter does set them). Budget is spent by counted failures, so
// publishing the remaining count on every response would tell anyone who can
// reach the endpoint how many failures a given key has accumulated.
func (m *middleware) block(c *fiber.Ctx, key string, reserved reservation) error {
	retryAfter := strconv.Itoa(retryAfterSeconds(reserved.retryAfter))
	c.Set(fiber.HeaderRetryAfter, retryAfter)
	c.Set(HeaderLimit, strconv.Itoa(m.max))
	c.Set(HeaderRemaining, "0")
	c.Set(HeaderReset, retryAfter)

	// Before the body: the handler never runs, so per-request resources it would
	// have released have to be released here.
	if m.onBlocked != nil {
		m.onBlocked(c)
	}

	if reserved.firstBlock {
		pkgLog.New().WithPkg("ratelimit").Warnf(
			"rate limit reached: scope=%q key=%q max=%d window=%s",
			m.scope, pkgText.SanitizeUntrusted(key, keyLogLimit), m.max, m.window,
		)
	}

	return pkgHttp.Err(c, pkgErr.TooManyRequests(m.message))
}

// responseStatus is the status this request will actually answer with.
//
// A handler that RETURNS an error has not written a response yet — Fiber's
// ErrorHandler runs after this middleware — so the recorded status is stale and
// the error has to be read instead. An unrecognized error becomes 500, which the
// default predicate does not charge for: an unknown failure is the server's
// problem, not the caller's.
func responseStatus(c *fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}

	var statusError pkgErr.Error
	if errors.As(err, &statusError) {
		return statusError.Status()
	}
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		return fiberError.Code
	}
	return http.StatusInternalServerError
}

// retryAfterSeconds rounds up to whole seconds, floored at 1: a Retry-After of 0
// reads as "retry immediately", which is the opposite of the instruction.
func retryAfterSeconds(remaining time.Duration) int {
	seconds := int(math.Ceil(remaining.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}
