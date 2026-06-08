package claims

import (
	"crypto/rand"
	"crypto/rsa"
	"slices"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func rsaKeyPair(t *testing.T) (priv *rsa.PrivateKey, pub *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key, &key.PublicKey
}

// signAndParse round-trips claims through RS256 sign + verify so the assertions
// run against JSON-decoded values ([]any / map[string]any), the same shapes the
// downstream getters must defensively handle.
func signAndParse(t *testing.T, c Claims) Claims {
	t.Helper()
	priv, pub := rsaKeyPair(t)

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c.MapClaims).SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	parsed := Claims{}
	if _, err := jwt.ParseWithClaims(tokenString, &parsed.MapClaims, func(*jwt.Token) (any, error) {
		return pub, nil
	}); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	return parsed
}

func TestResourceAccessRoundTrip(t *testing.T) {
	original := NewClaims("user-1", "user@example.com", 3600).
		WithAudience([]string{"isme", "medioa2"}).
		WithResourceAccess(map[string][]string{
			"isme":    {"user:read", "user:create"},
			"medioa2": {"object:read"},
		})

	parsed := signAndParse(t, original)

	if got := parsed.GetUserID(); got != "user-1" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-1")
	}
	if got := parsed.GetEmail(); got != "user@example.com" {
		t.Errorf("GetEmail() = %q, want %q", got, "user@example.com")
	}
	if got, want := parsed.GetPermsForApp("isme"), []string{"user:read", "user:create"}; !slices.Equal(got, want) {
		t.Errorf("GetPermsForApp(isme) = %v, want %v", got, want)
	}
	if got, want := parsed.GetPermsForApp("medioa2"), []string{"object:read"}; !slices.Equal(got, want) {
		t.Errorf("GetPermsForApp(medioa2) = %v, want %v", got, want)
	}
	if got := parsed.GetPermsForApp("rainy"); got != nil {
		t.Errorf("GetPermsForApp(rainy) = %v, want nil", got)
	}
	if !parsed.HasPermForApp("isme", "user:create") {
		t.Error("HasPermForApp(isme, user:create) = false, want true")
	}
	if parsed.HasPermForApp("isme", "object:read") {
		t.Error("HasPermForApp(isme, object:read) = true, want false")
	}
}

func TestGetAudienceArray(t *testing.T) {
	parsed := signAndParse(t, NewClaims("u", "e", 3600).WithAudience([]string{"isme", "rainy"}))
	got := parsed.GetAudience()
	slices.Sort(got)
	if want := []string{"isme", "rainy"}; !slices.Equal(got, want) {
		t.Errorf("GetAudience() = %v, want %v", got, want)
	}
}

func TestGetAudienceSingleString(t *testing.T) {
	// Per the JWT spec, aud may be a single string. Standard JWT libraries (and
	// some issuers) collapse a one-element audience to a bare string, so
	// GetAudience must accept that shape too.
	c := NewClaims("u", "e", 3600)
	c.MapClaims[AudienceKey] = "medioa2"
	parsed := signAndParse(t, c)
	if got, want := parsed.GetAudience(), []string{"medioa2"}; !slices.Equal(got, want) {
		t.Errorf("GetAudience() = %v, want %v", got, want)
	}
}

func TestGetPermsForAppMissingResourceAccess(t *testing.T) {
	parsed := signAndParse(t, NewClaims("u", "e", 3600))
	if got := parsed.GetPermsForApp("isme"); got != nil {
		t.Errorf("GetPermsForApp() = %v, want nil", got)
	}
	if got := parsed.GetAudience(); got != nil {
		t.Errorf("GetAudience() = %v, want nil", got)
	}
}
