package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http/httptest"
	"slices"
	"testing"

	pkgClaims "github.com/vukyn/kuery/claims"
	pkgCtx "github.com/vukyn/kuery/ctx"
	pkgJWT "github.com/vukyn/kuery/jwt"

	"github.com/gofiber/fiber/v2"
)

func rsaPEMPair(t *testing.T) (privPEM, pubPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	privPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal RSA public key: %v", err)
	}
	pubPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}))
	return privPEM, pubPEM
}

// newApp builds a Fiber app guarded by the auth middleware. The protected
// handler reports the resolved identity and perms so the test can assert what
// the middleware injected into the request context.
func newApp(pubPEM, appCode string) (*fiber.App, *capture) {
	cap := &capture{}
	app := fiber.New()
	app.Get("/protected", NewAuthMiddleware(pubPEM, appCode), func(c *fiber.Ctx) error {
		cap.userID = pkgCtx.GetUserIdFromFiberCtx(c)
		cap.email = pkgCtx.GetUserEmailFromFiberCtx(c)
		cap.tokenID = pkgCtx.GetTokenIDFromFiberCtx(c)
		cap.perms = pkgCtx.GetPermsFromFiberCtx(c)
		return c.SendStatus(fiber.StatusOK)
	})
	return app, cap
}

// newAppWithChecker is newApp for the revocation-aware constructor. It also
// records whether the guarded handler ran at all, which is what separates
// "middleware let the request through" from "middleware short-circuited".
func newAppWithChecker(pubPEM, appCode string, checker RevocationChecker) (*fiber.App, *capture) {
	cap := &capture{}
	app := fiber.New()
	app.Get("/protected", NewAuthMiddlewareWithRevocation(pubPEM, appCode, checker), func(c *fiber.Ctx) error {
		cap.reached = true
		cap.userID = pkgCtx.GetUserIdFromFiberCtx(c)
		cap.email = pkgCtx.GetUserEmailFromFiberCtx(c)
		cap.tokenID = pkgCtx.GetTokenIDFromFiberCtx(c)
		cap.sessionID = pkgCtx.GetSessionIDFromFiberCtx(c)
		cap.perms = pkgCtx.GetPermsFromFiberCtx(c)
		return c.SendStatus(fiber.StatusOK)
	})
	return app, cap
}

type capture struct {
	reached   bool
	userID    string
	email     string
	tokenID   string
	sessionID string
	perms     []string
}

// stubChecker stands in for a service's session store. It records the calls it
// received so a test can assert the middleware did NOT consult it — the "no sid"
// case is only meaningful if the checker was skipped, not merely if it answered
// "not revoked".
type stubChecker struct {
	revoked bool
	err     error

	calls        int
	gotSessionID string
	gotContext   context.Context
}

func (s *stubChecker) IsRevoked(ctx context.Context, sessionID string) (bool, error) {
	s.calls++
	s.gotSessionID = sessionID
	s.gotContext = ctx
	return s.revoked, s.err
}

func request(t *testing.T, app *fiber.App, authHeader string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set(fiber.HeaderAuthorization, authHeader)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp.StatusCode
}

func signToken(t *testing.T, privPEM string, audience []string, access map[string][]string) string {
	t.Helper()
	claims := pkgClaims.NewClaims("user-9", "user@example.com", 3600).
		WithAudience(audience).
		WithResourceAccess(access)
	token, err := pkgJWT.GenerateJWTWithRSAPrivateKeyFromClaims(privPEM, claims)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestAuthMiddlewareValidTokenMatchingAudience(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newApp(pubPEM, "medioa2")

	token := signToken(t, privPEM, []string{"medioa2"}, map[string][]string{
		"medioa2": {"object:read", "object:create"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if cap.userID != "user-9" {
		t.Errorf("userID = %q, want %q", cap.userID, "user-9")
	}
	if cap.email != "user@example.com" {
		t.Errorf("email = %q, want %q", cap.email, "user@example.com")
	}
	if cap.tokenID == "" {
		t.Error("tokenID is empty, want a value")
	}
	if want := []string{"object:read", "object:create"}; !slices.Equal(cap.perms, want) {
		t.Errorf("perms = %v, want %v", cap.perms, want)
	}
}

func TestAuthMiddlewareWrongAudienceForbidden(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, _ := newApp(pubPEM, "rainy")

	// token minted for medioa2 only; rainy must reject it.
	token := signToken(t, privPEM, []string{"medioa2"}, map[string][]string{
		"medioa2": {"object:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, fiber.StatusForbidden)
	}
}

func TestAuthMiddlewareBadSignatureUnauthorized(t *testing.T) {
	signingPriv, _ := rsaPEMPair(t)
	_, verifyingPub := rsaPEMPair(t) // different key pair => signature won't verify
	app, _ := newApp(verifyingPub, "medioa2")

	token := signToken(t, signingPriv, []string{"medioa2"}, map[string][]string{
		"medioa2": {"object:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, fiber.StatusUnauthorized)
	}
}

func TestAuthMiddlewareMissingHeaderUnauthorized(t *testing.T) {
	_, pubPEM := rsaPEMPair(t)
	app, _ := newApp(pubPEM, "medioa2")

	if status := request(t, app, ""); status != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, fiber.StatusUnauthorized)
	}
}

func TestAuthMiddlewareMalformedHeaderUnauthorized(t *testing.T) {
	_, pubPEM := rsaPEMPair(t)
	app, _ := newApp(pubPEM, "medioa2")

	if status := request(t, app, "Token abc"); status != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, fiber.StatusUnauthorized)
	}
}

const testSessionID = "01JBQ7Z2K9SESSION0000000"

// signTokenWithSession mints a valid token for appCode, carrying sessionID as the
// "sid" claim. An empty sessionID omits the claim entirely, which is how tokens
// minted before session tracking existed look.
func signTokenWithSession(t *testing.T, privPEM, appCode, sessionID string) string {
	t.Helper()
	claims := pkgClaims.NewClaims("user-9", "user@example.com", 3600).
		WithAudience([]string{appCode}).
		WithResourceAccess(map[string][]string{appCode: {"object:read"}})
	if sessionID != "" {
		claims = claims.WithSessionID(sessionID)
	}
	token, err := pkgJWT.GenerateJWTWithRSAPrivateKeyFromClaims(privPEM, claims)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

// TestRevocationNilCheckerPasses covers the path isme, medioa2 and rainy take:
// they pass no checker, so a valid token must be handled exactly as before even
// though it now carries a session id.
func TestRevocationNilCheckerPasses(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newAppWithChecker(pubPEM, "gardener", nil)

	token := signTokenWithSession(t, privPEM, "gardener", testSessionID)

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if !cap.reached {
		t.Fatal("handler did not run")
	}
	// The id must still land in the request context: a service can address the
	// session (e.g. to revoke it on logout) without opting into the check.
	if cap.sessionID != testSessionID {
		t.Errorf("sessionID = %q, want %q", cap.sessionID, testSessionID)
	}
}

// TestRevocationRevokedSessionUnauthorized is the whole point of the feature: a
// token that is still cryptographically valid and correctly addressed must be
// rejected once its session is gone.
func TestRevocationRevokedSessionUnauthorized(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	checker := &stubChecker{revoked: true}
	app, cap := newAppWithChecker(pubPEM, "gardener", checker)

	token := signTokenWithSession(t, privPEM, "gardener", testSessionID)

	if status := request(t, app, "Bearer "+token); status != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, fiber.StatusUnauthorized)
	}
	if cap.reached {
		t.Error("handler ran, want the middleware to short-circuit")
	}
	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}
	if checker.gotSessionID != testSessionID {
		t.Errorf("checker got session id %q, want %q", checker.gotSessionID, testSessionID)
	}
	// A nil context here would blow up any checker that derives a timeout from it.
	if checker.gotContext == nil {
		t.Error("checker got a nil context, want the request context")
	}
}

func TestRevocationActiveSessionPasses(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	checker := &stubChecker{revoked: false}
	app, cap := newAppWithChecker(pubPEM, "gardener", checker)

	token := signTokenWithSession(t, privPEM, "gardener", testSessionID)

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if !cap.reached {
		t.Fatal("handler did not run")
	}
	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}
}

// TestRevocationMissingSessionIDSkipsChecker covers tokens minted before the sid
// claim existed. Asserting the status alone would pass even if the middleware
// looked up the empty string and the checker happened to answer "not revoked" —
// so this asserts the checker was never consulted at all.
func TestRevocationMissingSessionIDSkipsChecker(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	// revoked: true so that any lookup at all would turn into a 401 and fail here.
	checker := &stubChecker{revoked: true}
	app, cap := newAppWithChecker(pubPEM, "gardener", checker)

	token := signTokenWithSession(t, privPEM, "gardener", "")

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if !cap.reached {
		t.Fatal("handler did not run")
	}
	if checker.calls != 0 {
		t.Errorf("checker calls = %d, want 0 (a token without sid must not be looked up)", checker.calls)
	}
}

// TestRevocationCheckerErrorPasses pins the ownership contract: the middleware
// holds no fail-open/fail-closed policy of its own, so an error means "continue".
// The checker is the only layer that can tell a transient backend blip from a
// real signal, and it must express fail-closed as (true, nil).
func TestRevocationCheckerErrorPasses(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	// revoked: true alongside the error, to pin that the bool is ignored when err
	// is non-nil rather than merely that a (false, err) answer passes.
	checker := &stubChecker{revoked: true, err: errors.New("session store unavailable")}
	app, cap := newAppWithChecker(pubPEM, "gardener", checker)

	token := signTokenWithSession(t, privPEM, "gardener", testSessionID)

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if !cap.reached {
		t.Fatal("handler did not run")
	}
	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}
}

// TestRevocationNotConsultedBeforeAudienceCheck pins the ordering: a token that
// fails the cheap local audience check must be rejected without paying for a
// session lookup (which in a real service is a cache miss away from a DB query).
func TestRevocationNotConsultedBeforeAudienceCheck(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	checker := &stubChecker{}
	app, _ := newAppWithChecker(pubPEM, "gardener", checker)

	// minted for a different app
	token := signTokenWithSession(t, privPEM, "rainy", testSessionID)

	if status := request(t, app, "Bearer "+token); status != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, fiber.StatusForbidden)
	}
	if checker.calls != 0 {
		t.Errorf("checker calls = %d, want 0 (audience is checked first)", checker.calls)
	}
}
