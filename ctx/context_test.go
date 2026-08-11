package ctx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"unsafe"

	pkgClaims "github.com/vukyn/kuery/claims"

	"github.com/gofiber/fiber/v2"
)

const testSessionID = "01JBQ7Z2K9SESSION0000000"

// TestSessionIDRoundTripsBothPaths covers the two ways a session id reaches
// application code: read straight back off the Fiber locals (middleware and
// handlers), and read off the context.Context that NewContextFromFiberCtx builds
// for usecases. Wiring only one of the two is the easy mistake — the handler
// would see the id and the usecase underneath it would silently see "".
func TestSessionIDRoundTripsBothPaths(t *testing.T) {
	app := fiber.New()

	var fromLocals, fromContext string
	app.Get("/", func(c *fiber.Ctx) error {
		SetSessionIDToFiberCtx(c, testSessionID)
		fromLocals = GetSessionIDFromFiberCtx(c)
		fromContext = GetSessionID(NewContextFromFiberCtx(c))
		return c.SendStatus(http.StatusOK)
	})

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if fromLocals != testSessionID {
		t.Errorf("GetSessionIDFromFiberCtx() = %q, want %q", fromLocals, testSessionID)
	}
	if fromContext != testSessionID {
		t.Errorf("GetSessionID(ctx) = %q, want %q", fromContext, testSessionID)
	}
}

// TestSetClaimsToFiberCtxCarriesSessionID pins the isme-internal claims path,
// which populates the locals from a freshly validated token rather than field by
// field.
func TestSetClaimsToFiberCtxCarriesSessionID(t *testing.T) {
	app := fiber.New()

	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		SetClaimsToFiberCtx(c, pkgClaims.NewClaims("u", "e", 3600).WithSessionID(testSessionID))
		got = GetSessionIDFromFiberCtx(c)
		return c.SendStatus(http.StatusOK)
	})

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if got != testSessionID {
		t.Errorf("session id after SetClaimsToFiberCtx = %q, want %q", got, testSessionID)
	}
}

// TestSessionIDAbsentIsEmpty covers both "nothing was ever set" entry points, so
// a caller can treat "" as "unknown session" instead of guarding against nil.
func TestSessionIDAbsentIsEmpty(t *testing.T) {
	if got := GetSessionID(context.Background()); got != "" {
		t.Errorf("GetSessionID(background) = %q, want %q", got, "")
	}

	app := fiber.New()
	var fromLocals string
	app.Get("/", func(c *fiber.Ctx) error {
		fromLocals = GetSessionIDFromFiberCtx(c)
		return c.SendStatus(http.StatusOK)
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if fromLocals != "" {
		t.Errorf("GetSessionIDFromFiberCtx() with nothing set = %q, want %q", fromLocals, "")
	}
}

// TestFiberOwnedStringsAreCloned pins that the two request-derived accessors return
// a COPY, not Fiber's own bytes.
//
// It compares backing-array pointers rather than values, because the values are
// equal either way — that is exactly what makes the bug invisible. Fiber hands out
// a zero-copy view into fasthttp's request buffer, which is reused for the next
// request on the same connection, so a caller retaining the value past the request
// silently watches it change. Remove the strings.Clone in either accessor and this
// test fails.
func TestFiberOwnedStringsAreCloned(t *testing.T) {
	const userAgent = "curl/8.7.1 test-agent"

	tests := []struct {
		name string
		// fiberOwned reads the value straight from Fiber; accessor reads it through
		// the kuery helper under test.
		fiberOwned func(*fiber.Ctx) string
		accessor   func(*fiber.Ctx) string
	}{
		{
			name:       "user agent",
			fiberOwned: func(c *fiber.Ctx) string { return c.Get("User-Agent") },
			accessor:   GetUserAgentFromFiberCtx,
		},
		{
			name:       "client ip",
			fiberOwned: func(c *fiber.Ctx) string { return c.IP() },
			accessor:   GetClientIPFromFiberCtx,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ProxyHeader puts c.IP() on the header path, where it is Fiber-owned; the
			// plain socket path allocates on its own and would not exercise this.
			app := fiber.New(fiber.Config{ProxyHeader: "X-Client-IP"})

			var failure string
			app.Get("/", func(c *fiber.Ctx) error {
				owned := tt.fiberOwned(c)
				cloned := tt.accessor(c)

				if owned != cloned {
					failure = "accessor changed the value, it must only copy it: " +
						owned + " != " + cloned
					return c.SendStatus(http.StatusOK)
				}
				if owned == "" {
					failure = "the test drove an empty value, so nothing was proven"
					return c.SendStatus(http.StatusOK)
				}
				// Equal contents, different backing array => a real copy.
				if unsafe.StringData(owned) == unsafe.StringData(cloned) {
					failure = "accessor returned Fiber's own bytes; it must clone them, " +
						"or a caller retaining the value gets a string that mutates"
				}
				return c.SendStatus(http.StatusOK)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("User-Agent", userAgent)
			request.Header.Set("X-Client-IP", "203.0.113.47")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer func() { _ = response.Body.Close() }()

			if failure != "" {
				t.Fatal(failure)
			}
		})
	}
}
