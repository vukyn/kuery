package apikey

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	pkgCryp "github.com/vukyn/kuery/cryp"
	pkgErr "github.com/vukyn/kuery/http/errors"
	pkgHttp "github.com/vukyn/kuery/http/fiberv3"
)

// Fixture credentials. Named constants rather than inline literals so nothing in
// this file reads like a real key, and so a secret scanner has a stable shape to
// allow-list.
const (
	primaryRawKey   = "primary-fixture-key-not-a-real-credential"
	secondaryRawKey = "secondary-fixture-key-not-a-real-credential"
	tertiaryRawKey  = "tertiary-fixture-key-not-a-real-credential"
	unknownRawKey   = "unknown-fixture-key-not-a-real-credential"

	primaryLabel   = "client-primary"
	secondaryLabel = "client-secondary"
	tertiaryLabel  = "client-tertiary"

	guardedPath   = "/guarded"
	referencePath = "/reference"
)

// capture records what the guarded handler saw, so a test can tell "the
// middleware let the request through" apart from "the middleware answered 200".
type capture struct {
	reached bool
	label   string
}

func hashOf(rawKey string) string {
	return pkgCryp.HashSHA256(rawKey)
}

// newGuardedApp mounts the middleware in front of a handler that reports the
// label the middleware resolved, read back through the exported Label accessor.
func newGuardedApp(cfg Config) (*fiber.App, *capture) {
	captured := &capture{}
	app := fiber.New()
	app.Get(guardedPath, New(cfg), func(c fiber.Ctx) error {
		captured.reached = true
		captured.label = Label(c)
		return c.SendStatus(fiber.StatusOK)
	})
	return app, captured
}

// doRequest sends one request. An empty headerName means DefaultHeader; an empty
// rawKey means the header is not sent at all.
func doRequest(t *testing.T, app *fiber.App, path, headerName, rawKey string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(fiber.MethodGet, path, nil)
	if headerName == "" {
		headerName = DefaultHeader
	}
	if rawKey != "" {
		request.Header.Set(headerName, rawKey)
	}
	response, err := app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// TestFirstConfiguredKeyIsAccepted and TestSecondConfiguredKeyIsAccepted are a
// pair on purpose: together they are the rotation guarantee. Testing only one of
// them would pass against a middleware that looked at a single key.
func TestFirstConfiguredKeyIsAccepted(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: hashOf(primaryRawKey), Label: primaryLabel},
		{Hash: hashOf(secondaryRawKey), Label: secondaryLabel},
	}})

	response := doRequest(t, app, guardedPath, "", primaryRawKey)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if !captured.reached {
		t.Fatal("handler did not run")
	}
	if captured.label != primaryLabel {
		t.Errorf("label = %q, want %q", captured.label, primaryLabel)
	}
}

func TestSecondConfiguredKeyIsAccepted(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: hashOf(primaryRawKey), Label: primaryLabel},
		{Hash: hashOf(secondaryRawKey), Label: secondaryLabel},
	}})

	response := doRequest(t, app, guardedPath, "", secondaryRawKey)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	// A different key must yield a different label, or the label is not actually
	// coming from the matched slot.
	if captured.label != secondaryLabel {
		t.Errorf("label = %q, want %q", captured.label, secondaryLabel)
	}
}

// TestLastConfiguredKeyIsAccepted is the no-early-exit proof. A loop that broke
// or returned on the first comparison would never reach the third slot, and a
// loop that stopped at the first MISS would fail here too.
func TestLastConfiguredKeyIsAccepted(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: hashOf(primaryRawKey), Label: primaryLabel},
		{Hash: hashOf(secondaryRawKey), Label: secondaryLabel},
		{Hash: hashOf(tertiaryRawKey), Label: tertiaryLabel},
	}})

	response := doRequest(t, app, guardedPath, "", tertiaryRawKey)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if captured.label != tertiaryLabel {
		t.Errorf("label = %q, want %q", captured.label, tertiaryLabel)
	}
}

func TestWrongKeyIsRejected(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: hashOf(primaryRawKey), Label: primaryLabel},
	}})

	response := doRequest(t, app, guardedPath, "", unknownRawKey)
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
	if captured.reached {
		t.Error("handler ran, want the middleware to short-circuit")
	}
}

func TestMissingHeaderIsRejected(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: hashOf(primaryRawKey), Label: primaryLabel},
	}})

	response := doRequest(t, app, guardedPath, "", "")
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
	if captured.reached {
		t.Error("handler ran, want the middleware to short-circuit")
	}
}

// TestEmptyKeysRejectEveryRequest is the highest-value case here: a deploy that
// lost its key configuration must end up with a locked door. A match loop that
// treated "nothing to compare against" as success would open the endpoint to
// anyone, silently.
func TestEmptyKeysRejectEveryRequest(t *testing.T) {
	app, captured := newGuardedApp(Config{})

	response := doRequest(t, app, guardedPath, "", primaryRawKey)
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
	if captured.reached {
		t.Fatal("handler ran with no keys configured; the guard must fail closed")
	}

	// ...and a request with no header at all must be refused the same way.
	response = doRequest(t, app, guardedPath, "", "")
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
}

// TestUpperCaseConfiguredHashMatches covers a digest pasted from a tool that
// emits upper-case hex. Without normalisation it would never match, and the
// failure would look like a wrong key rather than a wrong case.
func TestUpperCaseConfiguredHashMatches(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: strings.ToUpper(hashOf(primaryRawKey)), Label: primaryLabel},
	}})

	response := doRequest(t, app, guardedPath, "", primaryRawKey)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if captured.label != primaryLabel {
		t.Errorf("label = %q, want %q", captured.label, primaryLabel)
	}
}

// TestKeyWithoutLabelStillAuthenticates pins that the match itself is the
// decision. Tracking the match by "did we get a label" would reject a perfectly
// valid single-caller configuration.
func TestKeyWithoutLabelStillAuthenticates(t *testing.T) {
	app, captured := newGuardedApp(Config{Keys: []Key{
		{Hash: hashOf(primaryRawKey)},
	}})

	response := doRequest(t, app, guardedPath, "", primaryRawKey)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if !captured.reached {
		t.Fatal("handler did not run for a labelless key")
	}
	if captured.label != "" {
		t.Errorf("label = %q, want empty", captured.label)
	}
}

const (
	customHeader    = "X-Cron-Key"
	customLocalsKey = "cron_caller"
)

func TestCustomHeaderAndLocalsKey(t *testing.T) {
	captured := &capture{}
	app := fiber.New()
	app.Get(guardedPath, New(Config{
		Keys:      []Key{{Hash: hashOf(primaryRawKey), Label: primaryLabel}},
		Header:    customHeader,
		LocalsKey: customLocalsKey,
	}), func(c fiber.Ctx) error {
		captured.reached = true
		captured.label = LabelFrom(c, customLocalsKey)
		// The default accessor must find nothing: a custom locals key has to keep
		// this guard's label out of the default slot another guard reads.
		if leaked := Label(c); leaked != "" {
			t.Errorf("Label(c) = %q, want empty for a custom locals key", leaked)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	response := doRequest(t, app, guardedPath, customHeader, primaryRawKey)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if captured.label != primaryLabel {
		t.Errorf("label = %q, want %q", captured.label, primaryLabel)
	}

	// The default header must NOT be honoured once a custom one is configured.
	response = doRequest(t, app, guardedPath, DefaultHeader, primaryRawKey)
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d for a key sent in the wrong header", response.StatusCode, fiber.StatusUnauthorized)
	}
}

// TestRejectionMatchesUnauthorizedEnvelope compares the guard's 401 against the
// platform's own, produced by the same funnel, rather than against a hardcoded
// body — so the assertion keeps holding if the envelope is ever reshaped, and
// still fails the moment this package stops using the funnel.
func TestRejectionMatchesUnauthorizedEnvelope(t *testing.T) {
	app, _ := newGuardedApp(Config{Keys: []Key{{Hash: hashOf(primaryRawKey), Label: primaryLabel}}})
	app.Get(referencePath, func(c fiber.Ctx) error {
		return pkgHttp.Err(c, pkgErr.Unauthorized(DefaultMessage))
	})

	rejected := doRequest(t, app, guardedPath, "", unknownRawKey)
	reference := doRequest(t, app, referencePath, "", "")

	if rejected.StatusCode != reference.StatusCode {
		t.Errorf("status = %d, want %d", rejected.StatusCode, reference.StatusCode)
	}
	rejectedBody := readBody(t, rejected)
	referenceBody := readBody(t, reference)
	if rejectedBody != referenceBody {
		t.Errorf("body = %s, want %s", rejectedBody, referenceBody)
	}
}

// TestMissingHeaderAndWrongKeyAreIndistinguishable: the response must not tell a
// caller whether it guessed the header name right.
func TestMissingHeaderAndWrongKeyAreIndistinguishable(t *testing.T) {
	app, _ := newGuardedApp(Config{Keys: []Key{{Hash: hashOf(primaryRawKey), Label: primaryLabel}}})

	missing := doRequest(t, app, guardedPath, "", "")
	wrong := doRequest(t, app, guardedPath, "", unknownRawKey)

	if missing.StatusCode != wrong.StatusCode {
		t.Errorf("missing-header status %d != wrong-key status %d", missing.StatusCode, wrong.StatusCode)
	}
	if missingBody, wrongBody := readBody(t, missing), readBody(t, wrong); missingBody != wrongBody {
		t.Errorf("missing-header body %s != wrong-key body %s", missingBody, wrongBody)
	}
}

const maxKeysForTest = 2

func TestParseKeysAcceptsBareAndLabelledForms(t *testing.T) {
	raw := hashOf(primaryRawKey) + ":" + primaryLabel + ", " + hashOf(secondaryRawKey)

	keys, err := ParseKeys(raw, maxKeysForTest)
	if err != nil {
		t.Fatalf("ParseKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keys))
	}
	if keys[0].Hash != hashOf(primaryRawKey) || keys[0].Label != primaryLabel {
		t.Errorf("keys[0] = %+v, want the labelled form parsed", keys[0])
	}
	if keys[1].Hash != hashOf(secondaryRawKey) || keys[1].Label != "" {
		t.Errorf("keys[1] = %+v, want the bare form parsed with no label", keys[1])
	}
}

// TestParseKeysLowerCasesHashes: an upper-case digest in configuration must come
// out normalised, so the value stored is the value compared.
func TestParseKeysLowerCasesHashes(t *testing.T) {
	keys, err := ParseKeys(strings.ToUpper(hashOf(primaryRawKey))+":"+primaryLabel, 0)
	if err != nil {
		t.Fatalf("ParseKeys: %v", err)
	}
	if keys[0].Hash != hashOf(primaryRawKey) {
		t.Errorf("hash = %q, want it lower-cased", keys[0].Hash)
	}
}

func TestParseKeysRejectsShortEntry(t *testing.T) {
	if _, err := ParseKeys("abc123", 0); err == nil {
		t.Fatal("ParseKeys accepted an entry that is not 64 characters")
	}
}

func TestParseKeysRejectsNonHexEntry(t *testing.T) {
	nonHex := strings.Repeat("z", 64)
	if _, err := ParseKeys(nonHex, 0); err == nil {
		t.Fatal("ParseKeys accepted a 64-character non-hex entry")
	}
}

func TestParseKeysRejectsDuplicateHash(t *testing.T) {
	raw := hashOf(primaryRawKey) + ":" + primaryLabel + "," + hashOf(primaryRawKey) + ":" + secondaryLabel
	if _, err := ParseKeys(raw, 0); err == nil {
		t.Fatal("ParseKeys accepted a duplicate hash")
	}
}

func TestParseKeysRejectsMoreThanMax(t *testing.T) {
	raw := strings.Join([]string{
		hashOf(primaryRawKey),
		hashOf(secondaryRawKey),
		hashOf(tertiaryRawKey),
	}, ",")
	if _, err := ParseKeys(raw, maxKeysForTest); err == nil {
		t.Fatalf("ParseKeys accepted %d entries with max %d", 3, maxKeysForTest)
	}
	// ...and the same input is fine when no limit is asked for.
	keys, err := ParseKeys(raw, 0)
	if err != nil {
		t.Fatalf("ParseKeys with max 0: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("len(keys) = %d, want 3", len(keys))
	}
	if _, err := ParseKeys(raw, -1); err != nil {
		t.Errorf("ParseKeys with a negative max: %v, want no limit", err)
	}
}

func TestParseKeysEmptyStringYieldsNoKeys(t *testing.T) {
	keys, err := ParseKeys("   ", 0)
	if err != nil {
		t.Fatalf("ParseKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("len(keys) = %d, want 0", len(keys))
	}
	// An empty configuration is not an open door — New refuses everything.
	app, _ := newGuardedApp(Config{Keys: keys})
	if response := doRequest(t, app, guardedPath, "", primaryRawKey); response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
}

// TestParseKeysErrorNeverEchoesTheValue: the likeliest reason an entry is
// malformed is that a REAL key was pasted where its digest belonged, and error
// text ends up in logs.
func TestParseKeysErrorNeverEchoesTheValue(t *testing.T) {
	_, err := ParseKeys(primaryRawKey, 0)
	if err == nil {
		t.Fatal("ParseKeys accepted a raw key as a hash")
	}
	if strings.Contains(err.Error(), primaryRawKey) {
		t.Errorf("error %q echoes the offending value", err.Error())
	}

	nonHex := strings.Repeat("z", 64)
	_, err = ParseKeys(nonHex, 0)
	if err == nil {
		t.Fatal("ParseKeys accepted a non-hex hash")
	}
	if strings.Contains(err.Error(), nonHex) {
		t.Errorf("error %q echoes the offending value", err.Error())
	}
}

// TestParsedKeysAuthenticate closes the loop: what ParseKeys produces is what New
// accepts. Either half can be right on its own while the pair is broken.
func TestParsedKeysAuthenticate(t *testing.T) {
	keys, err := ParseKeys(hashOf(primaryRawKey)+":"+primaryLabel+","+hashOf(secondaryRawKey)+":"+secondaryLabel, 0)
	if err != nil {
		t.Fatalf("ParseKeys: %v", err)
	}

	app, captured := newGuardedApp(Config{Keys: keys})
	if response := doRequest(t, app, guardedPath, "", secondaryRawKey); response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if captured.label != secondaryLabel {
		t.Errorf("label = %q, want %q", captured.label, secondaryLabel)
	}
}

// TestLabelOnAnUnguardedRequestIsEmpty pins that the accessor identifies a caller
// and never proves one — a handler must not read a label as "authenticated".
func TestLabelOnAnUnguardedRequestIsEmpty(t *testing.T) {
	app := fiber.New()
	app.Get(guardedPath, func(c fiber.Ctx) error {
		if label := Label(c); label != "" {
			t.Errorf("Label(c) = %q on an unguarded route, want empty", label)
		}
		return c.SendStatus(http.StatusOK)
	})
	if response := doRequest(t, app, guardedPath, "", primaryRawKey); response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
}
