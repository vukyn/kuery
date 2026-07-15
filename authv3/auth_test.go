package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http/httptest"
	"slices"
	"testing"

	pkgClaims "github.com/vukyn/kuery/claims"
	pkgCtx "github.com/vukyn/kuery/ctxv3"
	pkgJWT "github.com/vukyn/kuery/jwt"

	"github.com/gofiber/fiber/v3"
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
	app.Get("/protected", NewAuthMiddleware(pubPEM, appCode), func(c fiber.Ctx) error {
		cap.userID = pkgCtx.GetUserIdFromFiberCtx(c)
		cap.email = pkgCtx.GetUserEmailFromFiberCtx(c)
		cap.tokenID = pkgCtx.GetTokenIDFromFiberCtx(c)
		cap.perms = pkgCtx.GetPermsFromFiberCtx(c)
		return c.SendStatus(fiber.StatusOK)
	})
	return app, cap
}

type capture struct {
	userID  string
	email   string
	tokenID string
	perms   []string
}

func request(t *testing.T, app *fiber.App, authHeader string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set(fiber.HeaderAuthorization, authHeader)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
