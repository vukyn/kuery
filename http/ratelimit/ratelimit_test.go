package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	pkgErr "github.com/vukyn/kuery/http/errors"
)

// fakeClock is a manually advanced clock, so a window-expiry test never sleeps
// for a real window.
type fakeClock struct {
	mutex sync.Mutex
	at    time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{at: time.Unix(1_700_000_000, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.at
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.at = c.at.Add(d)
}

// countingHandler answers a fixed status and records how many times it actually
// ran, so a test can PROVE a blocked request never reached the handler instead of
// inferring it from the status.
type countingHandler struct {
	status int
	calls  atomic.Int64
}

func (h *countingHandler) handle(c *fiber.Ctx) error {
	h.calls.Add(1)
	return c.Status(h.status).JSON(fiber.Map{"code": h.status})
}

// fixedKey is the simplest KeyFunc: one bucket for the whole test.
func fixedKey(key string) func(*fiber.Ctx) string {
	return func(*fiber.Ctx) string { return key }
}

// headerKey buckets on a request header, so one app can exercise several keys.
func headerKey(c *fiber.Ctx) string {
	return c.Get("X-Test-Key")
}

// newTestApp mounts the middleware in front of a counting handler on GET /.
func newTestApp(t *testing.T, cfg Config, status int) (*fiber.App, *countingHandler) {
	t.Helper()
	app := fiber.New()
	handler := &countingHandler{status: status}
	app.Get("/", New(cfg), handler.handle)
	return app, handler
}

// do issues one request and returns the response status.
func do(t *testing.T, app *fiber.App, key string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(fiber.MethodGet, "/", nil)
	if key != "" {
		request.Header.Set("X-Test-Key", key)
	}
	response, err := app.Test(request, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return response
}

// countFor reads a key's current counter, for the bookkeeping a response cannot
// show.
func countFor(t *testing.T, keys *store, key string) int {
	t.Helper()
	keys.mutex.Lock()
	defer keys.mutex.Unlock()
	current, found := keys.entries[key]
	if !found {
		return 0
	}
	return current.count
}

func storeSize(t *testing.T, keys *store) int {
	t.Helper()
	keys.mutex.Lock()
	defer keys.mutex.Unlock()
	return len(keys.entries)
}

func TestMaxFailuresThenBlockedWithoutReachingHandler(t *testing.T) {
	const max = 3
	app, handler := newTestApp(t, Config{
		Max:     max,
		Window:  time.Minute,
		KeyFunc: fixedKey("caller"),
		Scope:   "test",
	}, fiber.StatusUnauthorized)

	for attempt := 1; attempt <= max; attempt++ {
		response := do(t, app, "")
		if response.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("attempt %d: got status %d, want %d", attempt, response.StatusCode, fiber.StatusUnauthorized)
		}
	}

	blocked := do(t, app, "")
	if blocked.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("got status %d after %d failures, want %d", blocked.StatusCode, max, fiber.StatusTooManyRequests)
	}

	// The point of the assertion: the handler was skipped, not merely overruled.
	if calls := handler.calls.Load(); calls != max {
		t.Fatalf("handler ran %d times, want %d — a blocked request reached the handler", calls, max)
	}

	// Headers a client needs to back off.
	if retryAfter := blocked.Header.Get(fiber.HeaderRetryAfter); retryAfter == "" {
		t.Error("missing Retry-After header")
	} else if seconds, err := strconv.Atoi(retryAfter); err != nil || seconds < 1 {
		t.Errorf("Retry-After = %q, want whole seconds >= 1 (err %v)", retryAfter, err)
	}
	if got := blocked.Header.Get(HeaderLimit); got != strconv.Itoa(max) {
		t.Errorf("%s = %q, want %q", HeaderLimit, got, strconv.Itoa(max))
	}
	if got := blocked.Header.Get(HeaderRemaining); got != "0" {
		t.Errorf("%s = %q, want %q", HeaderRemaining, got, "0")
	}
	if got := blocked.Header.Get(HeaderReset); got == "" {
		t.Errorf("%s is empty", HeaderReset)
	}
}

// TestCountStatusDecidesWhoSpendsBudget is the headline fix: a 500 (database
// outage) or a 400 (malformed body) must never turn into a lockout.
func TestCountStatusDecidesWhoSpendsBudget(t *testing.T) {
	const max = 2
	const attempts = max + 5

	testCases := []struct {
		name        string
		status      int
		countStatus func(int) bool
		wantBlock   bool
	}{
		{name: "401 counts", status: fiber.StatusUnauthorized, wantBlock: true},
		{name: "403 counts", status: fiber.StatusForbidden, wantBlock: true},
		{name: "500 never counts", status: fiber.StatusInternalServerError, wantBlock: false},
		{name: "503 never counts", status: fiber.StatusServiceUnavailable, wantBlock: false},
		{name: "400 never counts by default", status: fiber.StatusBadRequest, wantBlock: false},
		{name: "408 never counts by default", status: fiber.StatusRequestTimeout, wantBlock: false},
		{name: "200 never counts", status: fiber.StatusOK, wantBlock: false},
		{name: "400 counts under CountClientErrors", status: fiber.StatusBadRequest, countStatus: CountClientErrors, wantBlock: true},
		{name: "408 still exempt under CountClientErrors", status: fiber.StatusRequestTimeout, countStatus: CountClientErrors, wantBlock: false},
		{name: "500 still exempt under CountClientErrors", status: fiber.StatusInternalServerError, countStatus: CountClientErrors, wantBlock: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			app, handler := newTestApp(t, Config{
				Max:         max,
				Window:      time.Minute,
				KeyFunc:     fixedKey("caller"),
				CountStatus: testCase.countStatus,
			}, testCase.status)

			blocked := false
			for attempt := 1; attempt <= attempts; attempt++ {
				if do(t, app, "").StatusCode == fiber.StatusTooManyRequests {
					blocked = true
					break
				}
			}

			if blocked != testCase.wantBlock {
				t.Fatalf("blocked = %v after %d responses of status %d, want %v", blocked, attempts, testCase.status, testCase.wantBlock)
			}
			if !testCase.wantBlock && handler.calls.Load() != int64(attempts) {
				t.Fatalf("handler ran %d/%d times — budget was spent on an uncounted status", handler.calls.Load(), attempts)
			}
		})
	}
}

// TestInnerBlockDoesNotChargeOuterTier is the ordering-trap removal. Fiber counts
// a 429 as a 4xx, so with its limiter one user blocked by the tight tier keeps
// charging the shared-address tier. Here the outer budget must be untouched.
func TestInnerBlockDoesNotChargeOuterTier(t *testing.T) {
	const outerMax = 5
	const innerMax = 1

	outerHandler, outerKeys := newMiddleware(Config{
		Max:     outerMax,
		Window:  time.Minute,
		KeyFunc: fixedKey("shared-address"),
		Scope:   "ip",
	})
	innerHandler, innerKeys := newMiddleware(Config{
		Max:     innerMax,
		Window:  time.Minute,
		KeyFunc: fixedKey("shared-address|victim"),
		Scope:   "ip+username",
	})

	app := fiber.New()
	handler := &countingHandler{status: fiber.StatusUnauthorized}
	app.Get("/", outerHandler, innerHandler, handler.handle)

	// One genuine failure: charges both tiers.
	if got := do(t, app, "").StatusCode; got != fiber.StatusUnauthorized {
		t.Fatalf("first attempt: got %d, want %d", got, fiber.StatusUnauthorized)
	}
	if got := countFor(t, outerKeys, "shared-address"); got != innerMax {
		t.Fatalf("outer count after one real failure = %d, want %d", got, innerMax)
	}

	// Everything after this is refused by the INNER tier with a 429.
	const floods = 20
	for attempt := 1; attempt <= floods; attempt++ {
		if got := do(t, app, "").StatusCode; got != fiber.StatusTooManyRequests {
			t.Fatalf("flood attempt %d: got %d, want %d", attempt, got, fiber.StatusTooManyRequests)
		}
	}

	// The assertion that matters: 429s did not drain the shared-address budget.
	if got := countFor(t, outerKeys, "shared-address"); got != 1 {
		t.Fatalf("outer count = %d after %d inner-blocked requests, want 1 — a 429 charged the outer tier", got, floods)
	}
	if got := countFor(t, innerKeys, "shared-address|victim"); got != innerMax {
		t.Fatalf("inner count = %d, want it pinned at %d", got, innerMax)
	}
	if calls := handler.calls.Load(); calls != 1 {
		t.Fatalf("handler ran %d times, want 1", calls)
	}

	// So the other users on that shared address still have almost the whole budget.
	if remaining := outerMax - countFor(t, outerKeys, "shared-address"); remaining != outerMax-1 {
		t.Fatalf("outer remaining = %d, want %d", remaining, outerMax-1)
	}
}

// TestKeysHaveIndependentBudgets doubles as the regression test for mutable keys:
// headerKey returns c.Get(...), which in Fiber v2 is a zero-copy string into
// fasthttp's recycled request buffer. Without the strings.Clone in handle, the map
// key mutates after the response and every request looks like a brand-new key —
// alice gets an endless budget and the map grows without bound.
func TestKeysHaveIndependentBudgets(t *testing.T) {
	const max = 2

	middlewareHandler, keys := newMiddleware(Config{
		Max:     max,
		Window:  time.Minute,
		KeyFunc: headerKey,
	})
	app := fiber.New()
	handler := &countingHandler{status: fiber.StatusUnauthorized}
	app.Get("/", middlewareHandler, handler.handle)

	// Spend the first key's whole budget.
	for attempt := 1; attempt <= max; attempt++ {
		if got := do(t, app, "alice").StatusCode; got != fiber.StatusUnauthorized {
			t.Fatalf("alice attempt %d: got %d", attempt, got)
		}
	}
	if got := do(t, app, "alice").StatusCode; got != fiber.StatusTooManyRequests {
		t.Fatalf("alice: got %d, want %d", got, fiber.StatusTooManyRequests)
	}

	// A different key is unaffected...
	for attempt := 1; attempt <= max; attempt++ {
		if got := do(t, app, "bob").StatusCode; got != fiber.StatusUnauthorized {
			t.Fatalf("bob attempt %d: got %d, want %d (alice's block leaked)", attempt, got, fiber.StatusUnauthorized)
		}
	}
	// ...until it spends its own.
	if got := do(t, app, "bob").StatusCode; got != fiber.StatusTooManyRequests {
		t.Fatalf("bob: got %d, want %d", got, fiber.StatusTooManyRequests)
	}

	// The same key keeps sharing one bucket across requests.
	if got := do(t, app, "alice").StatusCode; got != fiber.StatusTooManyRequests {
		t.Fatalf("alice again: got %d, want %d (the key did not survive the request)", got, fiber.StatusTooManyRequests)
	}
	if size := storeSize(t, keys); size != 2 {
		t.Fatalf("tracked %d keys for 2 callers, want 2 — request-scoped keys are leaking into the map", size)
	}
	if got := countFor(t, keys, "alice"); got != max {
		t.Fatalf("alice count = %d, want it pinned at %d", got, max)
	}
}

func TestWindowExpiryRestoresBudget(t *testing.T) {
	const max = 2
	const window = 5 * time.Minute
	clock := newFakeClock()

	app := fiber.New()
	handler := &countingHandler{status: fiber.StatusUnauthorized}
	app.Get("/", New(Config{
		Max:     max,
		Window:  window,
		KeyFunc: fixedKey("caller"),
		Clock:   clock.Now,
	}), handler.handle)

	for attempt := 1; attempt <= max; attempt++ {
		if got := do(t, app, "").StatusCode; got != fiber.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d", attempt, got)
		}
	}
	if got := do(t, app, "").StatusCode; got != fiber.StatusTooManyRequests {
		t.Fatalf("got %d, want %d", got, fiber.StatusTooManyRequests)
	}

	// Still inside the window: still blocked.
	clock.Advance(window - time.Second)
	if got := do(t, app, "").StatusCode; got != fiber.StatusTooManyRequests {
		t.Fatalf("inside window: got %d, want %d", got, fiber.StatusTooManyRequests)
	}

	// Window rolled over: budget is back, no sleeping involved.
	clock.Advance(2 * time.Second)
	for attempt := 1; attempt <= max; attempt++ {
		if got := do(t, app, "").StatusCode; got != fiber.StatusUnauthorized {
			t.Fatalf("new window attempt %d: got %d, want %d", attempt, got, fiber.StatusUnauthorized)
		}
	}
	if got := do(t, app, "").StatusCode; got != fiber.StatusTooManyRequests {
		t.Fatalf("new window exhausted: got %d, want %d", got, fiber.StatusTooManyRequests)
	}
}

func TestPassThroughConfigurations(t *testing.T) {
	testCases := []struct {
		name string
		cfg  Config
	}{
		{name: "max zero", cfg: Config{Max: 0, KeyFunc: fixedKey("caller")}},
		{name: "max negative", cfg: Config{Max: -1, KeyFunc: fixedKey("caller")}},
		{name: "nil key func", cfg: Config{Max: 1, KeyFunc: nil}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			app, handler := newTestApp(t, testCase.cfg, fiber.StatusUnauthorized)

			const attempts = 10
			for attempt := 1; attempt <= attempts; attempt++ {
				if got := do(t, app, "").StatusCode; got != fiber.StatusUnauthorized {
					t.Fatalf("attempt %d: got %d, want pass-through %d", attempt, got, fiber.StatusUnauthorized)
				}
			}
			if calls := handler.calls.Load(); calls != attempts {
				t.Fatalf("handler ran %d/%d times, want every request to pass through", calls, attempts)
			}
		})
	}
}

func TestOnBlockedRunsOncePerBlockedRequestBeforeBody(t *testing.T) {
	const max = 1
	var hookCalls atomic.Int64
	var bodyAlreadyWritten atomic.Int64

	app, handler := newTestApp(t, Config{
		Max:     max,
		Window:  time.Minute,
		KeyFunc: fixedKey("caller"),
		OnBlocked: func(c *fiber.Ctx) {
			hookCalls.Add(1)
			// The hook exists to release what the handler would have released, so
			// it has to run before the response is committed.
			if len(c.Response().Body()) > 0 || c.Response().StatusCode() == fiber.StatusTooManyRequests {
				bodyAlreadyWritten.Add(1)
			}
		},
	}, fiber.StatusUnauthorized)

	// Spend the budget: the hook must not fire for an allowed request.
	if got := do(t, app, "").StatusCode; got != fiber.StatusUnauthorized {
		t.Fatalf("first attempt: got %d", got)
	}
	if calls := hookCalls.Load(); calls != 0 {
		t.Fatalf("OnBlocked ran %d times for an allowed request, want 0", calls)
	}

	const blockedRequests = 3
	for attempt := 1; attempt <= blockedRequests; attempt++ {
		if got := do(t, app, "").StatusCode; got != fiber.StatusTooManyRequests {
			t.Fatalf("blocked attempt %d: got %d", attempt, got)
		}
	}

	if calls := hookCalls.Load(); calls != blockedRequests {
		t.Fatalf("OnBlocked ran %d times for %d blocked requests, want exactly once each", calls, blockedRequests)
	}
	if late := bodyAlreadyWritten.Load(); late != 0 {
		t.Fatalf("OnBlocked ran after the 429 was written %d times, want 0", late)
	}
	if calls := handler.calls.Load(); calls != 1 {
		t.Fatalf("handler ran %d times, want 1", calls)
	}
}

// TestBlockLoggedOncePerKeyPerWindow asserts the bookkeeping behind the log line.
// kuery/log has no injectable sink (it writes through the zerolog global), so
// there is no seam to count emitted lines without inventing one — the store's
// firstBlock/logged flag is the thing that decides, so that is what is asserted.
func TestBlockLoggedOncePerKeyPerWindow(t *testing.T) {
	const max = 1
	const window = time.Minute
	clock := newFakeClock()
	keys := newStore(max, window, DefaultMaxKeys, clock.Now)

	if reserved := keys.reserve("caller"); !reserved.allowed {
		t.Fatalf("first reserve was blocked, want allowed")
	}

	first := keys.reserve("caller")
	if first.allowed || !first.firstBlock {
		t.Fatalf("first block: allowed=%v firstBlock=%v, want false/true", first.allowed, first.firstBlock)
	}

	for attempt := 1; attempt <= 50; attempt++ {
		repeat := keys.reserve("caller")
		if repeat.allowed {
			t.Fatalf("repeat %d was allowed, want blocked", attempt)
		}
		if repeat.firstBlock {
			t.Fatalf("repeat %d reported firstBlock — a blocked flood would log per request", attempt)
		}
	}

	// The counter stays pinned instead of climbing with the flood.
	if got := countFor(t, keys, "caller"); got != max {
		t.Fatalf("count = %d after a flood, want it pinned at %d", got, max)
	}

	// A different key logs its own first block.
	other := keys.reserve("other")
	if !other.allowed {
		t.Fatalf("other key was blocked, want allowed")
	}
	if blocked := keys.reserve("other"); !blocked.firstBlock {
		t.Fatal("other key's first block did not report firstBlock")
	}

	// A new window logs again — the flag resets with the window.
	clock.Advance(window + time.Second)
	if reserved := keys.reserve("caller"); !reserved.allowed {
		t.Fatal("new window: reserve was blocked, want allowed")
	}
	if blocked := keys.reserve("caller"); !blocked.firstBlock {
		t.Fatal("new window: first block did not report firstBlock")
	}
}

func TestMaxKeysBoundsTrackedSet(t *testing.T) {
	const maxKeys = 8
	clock := newFakeClock()
	keys := newStore(3, time.Hour, maxKeys, clock.Now)

	// Far more distinct keys than the cap, with a clock that never advances, so
	// nothing expires and only eviction can keep the map bounded.
	for index := 0; index < 5_000; index++ {
		keys.reserve("key-" + strconv.Itoa(index))
		if size := storeSize(t, keys); size > maxKeys {
			t.Fatalf("tracked %d keys after %d distinct keys, want <= %d", size, index+1, maxKeys)
		}
	}
	if size := storeSize(t, keys); size == 0 {
		t.Fatal("tracked set is empty, want it bounded but populated")
	}

	// Saturation must not lock anyone out: a fresh key is still allowed.
	if reserved := keys.reserve("newcomer"); !reserved.allowed {
		t.Fatal("a new key was blocked while the map was full — the limiter failed closed")
	}

	// The same through the middleware, to prove no panic on the request path.
	app, _ := newTestApp(t, Config{
		Max:     2,
		Window:  time.Hour,
		MaxKeys: maxKeys,
		KeyFunc: headerKey,
	}, fiber.StatusUnauthorized)
	for index := 0; index < 200; index++ {
		if got := do(t, app, "caller-"+strconv.Itoa(index)).StatusCode; got != fiber.StatusUnauthorized {
			t.Fatalf("distinct key %d: got %d, want %d", index, got, fiber.StatusUnauthorized)
		}
	}
}

func TestConcurrentRequestsNeverExceedMax(t *testing.T) {
	const max = 10
	const goroutines = 40

	app, handler := newTestApp(t, Config{
		Max:     max,
		Window:  time.Minute,
		KeyFunc: fixedKey("caller"),
	}, fiber.StatusUnauthorized)

	var allowed atomic.Int64
	var blocked atomic.Int64
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)
	start := make(chan struct{})

	for index := 0; index < goroutines; index++ {
		go func() {
			defer waitGroup.Done()
			<-start
			request := httptest.NewRequest(fiber.MethodGet, "/", nil)
			response, err := app.Test(request, 10_000)
			if err != nil {
				t.Errorf("app.Test: %v", err)
				return
			}
			switch response.StatusCode {
			case fiber.StatusUnauthorized:
				allowed.Add(1)
			case fiber.StatusTooManyRequests:
				blocked.Add(1)
			default:
				t.Errorf("unexpected status %d", response.StatusCode)
			}
		}()
	}
	close(start)
	waitGroup.Wait()

	if got := allowed.Load(); got != max {
		t.Fatalf("%d requests were allowed, want exactly %d", got, max)
	}
	if got := blocked.Load(); got != goroutines-max {
		t.Fatalf("%d requests were blocked, want %d", got, goroutines-max)
	}
	if calls := handler.calls.Load(); calls != max {
		t.Fatalf("handler ran %d times, want %d", calls, max)
	}
}

// TestHandlerReturnedErrorIsNotCharged covers the other half of the outage fix: a
// handler that RETURNS an error (Fiber's ErrorHandler writes the 500 later) must
// not spend budget either.
func TestHandlerReturnedErrorIsNotCharged(t *testing.T) {
	testCases := []struct {
		name      string
		err       error
		wantBlock bool
	}{
		{name: "plain error becomes 500", err: errFailedDependency, wantBlock: false},
		{name: "fiber 500", err: fiber.NewError(fiber.StatusInternalServerError, "db down"), wantBlock: false},
		{name: "fiber 401 still counts", err: fiber.NewError(fiber.StatusUnauthorized, "bad credentials"), wantBlock: true},
		{name: "kuery database error", err: pkgErr.DatabaseError("connection refused"), wantBlock: false},
		{name: "kuery unauthorized still counts", err: pkgErr.Unauthorized("bad credentials"), wantBlock: true},
		{name: "kuery too many requests is never charged", err: pkgErr.TooManyRequests("slow down"), wantBlock: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			const max = 2
			const attempts = max + 4

			app := fiber.New()
			app.Get("/", New(Config{
				Max:     max,
				Window:  time.Minute,
				KeyFunc: fixedKey("caller"),
			}), func(*fiber.Ctx) error {
				return testCase.err
			})

			blocked := false
			for attempt := 1; attempt <= attempts; attempt++ {
				if do(t, app, "").StatusCode == fiber.StatusTooManyRequests {
					blocked = true
					break
				}
			}
			if blocked != testCase.wantBlock {
				t.Fatalf("blocked = %v, want %v", blocked, testCase.wantBlock)
			}
		})
	}
}

// errFailedDependency stands in for any error a handler can return that carries no
// HTTP status — a driver error, say.
var errFailedDependency = &plainError{message: "database is down"}

type plainError struct {
	message string
}

func (e *plainError) Error() string {
	return e.message
}

func TestStatusPredicates(t *testing.T) {
	testCases := []struct {
		status       int
		authFailures bool
		clientErrors bool
	}{
		{status: http.StatusOK, authFailures: false, clientErrors: false},
		{status: http.StatusBadRequest, authFailures: false, clientErrors: true},
		{status: http.StatusUnauthorized, authFailures: true, clientErrors: true},
		{status: http.StatusForbidden, authFailures: true, clientErrors: true},
		{status: http.StatusNotFound, authFailures: false, clientErrors: true},
		{status: http.StatusRequestTimeout, authFailures: false, clientErrors: false},
		{status: http.StatusTooManyRequests, authFailures: false, clientErrors: false},
		{status: http.StatusInternalServerError, authFailures: false, clientErrors: false},
		{status: http.StatusServiceUnavailable, authFailures: false, clientErrors: false},
	}

	for _, testCase := range testCases {
		t.Run(strconv.Itoa(testCase.status), func(t *testing.T) {
			if got := CountAuthFailures(testCase.status); got != testCase.authFailures {
				t.Errorf("CountAuthFailures(%d) = %v, want %v", testCase.status, got, testCase.authFailures)
			}
			if got := CountClientErrors(testCase.status); got != testCase.clientErrors {
				t.Errorf("CountClientErrors(%d) = %v, want %v", testCase.status, got, testCase.clientErrors)
			}
		})
	}
}

func TestRetryAfterSecondsRoundsUpAndFloorsAtOne(t *testing.T) {
	testCases := []struct {
		remaining time.Duration
		want      int
	}{
		{remaining: -time.Second, want: 1},
		{remaining: 0, want: 1},
		{remaining: time.Millisecond, want: 1},
		{remaining: time.Second, want: 1},
		{remaining: 1500 * time.Millisecond, want: 2},
		{remaining: 5 * time.Minute, want: 300},
	}

	for _, testCase := range testCases {
		if got := retryAfterSeconds(testCase.remaining); got != testCase.want {
			t.Errorf("retryAfterSeconds(%s) = %d, want %d", testCase.remaining, got, testCase.want)
		}
	}
}

func TestDefaultsAreApplied(t *testing.T) {
	_, keys := newMiddleware(Config{
		Max:     1,
		KeyFunc: fixedKey("caller"),
	})
	if keys == nil {
		t.Fatal("store is nil, want a configured limiter")
	}
	if keys.window != DefaultWindow {
		t.Errorf("window = %s, want %s", keys.window, DefaultWindow)
	}
	if keys.maxKeys != DefaultMaxKeys {
		t.Errorf("maxKeys = %d, want %d", keys.maxKeys, DefaultMaxKeys)
	}

	// A negative window falls back too — a misconfigured window must not mean
	// "no window".
	_, negative := newMiddleware(Config{
		Max:     1,
		Window:  -time.Minute,
		MaxKeys: -5,
		KeyFunc: fixedKey("caller"),
	})
	if negative.window != DefaultWindow {
		t.Errorf("window = %s, want %s", negative.window, DefaultWindow)
	}
	if negative.maxKeys != DefaultMaxKeys {
		t.Errorf("maxKeys = %d, want %d", negative.maxKeys, DefaultMaxKeys)
	}
}
