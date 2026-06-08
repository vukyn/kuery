package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"slices"
	"testing"

	pkgClaims "github.com/vukyn/kuery/claims"
)

func buildTestClaims() pkgClaims.Claims {
	return pkgClaims.NewClaims("user-1", "user@example.com", 3600).
		WithAudience([]string{"isme", "medioa2"}).
		WithResourceAccess(map[string][]string{
			"isme":    {"user:read", "role:read"},
			"medioa2": {"object:read"},
		})
}

func assertClaimsSurvived(t *testing.T, parsed pkgClaims.Claims) {
	t.Helper()
	if got := parsed.GetUserID(); got != "user-1" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-1")
	}
	if got := parsed.GetEmail(); got != "user@example.com" {
		t.Errorf("GetEmail() = %q, want %q", got, "user@example.com")
	}
	if got, want := parsed.GetAudience(), []string{"isme", "medioa2"}; !slices.Equal(sorted(got), sorted(want)) {
		t.Errorf("GetAudience() = %v, want %v", got, want)
	}
	if got, want := parsed.GetPermsForApp("isme"), []string{"user:read", "role:read"}; !slices.Equal(got, want) {
		t.Errorf("GetPermsForApp(isme) = %v, want %v", got, want)
	}
	if got, want := parsed.GetPermsForApp("medioa2"), []string{"object:read"}; !slices.Equal(got, want) {
		t.Errorf("GetPermsForApp(medioa2) = %v, want %v", got, want)
	}
	if !parsed.HasPermForApp("isme", "user:read") {
		t.Error("HasPermForApp(isme, user:read) = false, want true")
	}
	if parsed.HasPermForApp("medioa2", "user:read") {
		t.Error("HasPermForApp(medioa2, user:read) = true, want false")
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

func TestGenerateJWTFromClaimsRoundTrip(t *testing.T) {
	secretKey := "test-secret"

	tokenString, err := GenerateJWTFromClaims(secretKey, buildTestClaims())
	if err != nil {
		t.Fatalf("GenerateJWTFromClaims() error = %v", err)
	}

	parsed, err := ValidateJWT(tokenString, secretKey)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}
	assertClaimsSurvived(t, parsed)

	if _, err := ValidateJWT(tokenString, "wrong-secret"); err == nil {
		t.Error("ValidateJWT() with wrong secret expected error, got nil")
	}
}

func TestGenerateJWTWithRSAPrivateKeyFromClaimsRoundTrip(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}))
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal RSA public key: %v", err)
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	}))

	tokenString, err := GenerateJWTWithRSAPrivateKeyFromClaims(privateKeyPEM, buildTestClaims())
	if err != nil {
		t.Fatalf("GenerateJWTWithRSAPrivateKeyFromClaims() error = %v", err)
	}

	parsed, err := ValidateJWTWithRSAPublicKey(tokenString, publicKeyPEM)
	if err != nil {
		t.Fatalf("ValidateJWTWithRSAPublicKey() error = %v", err)
	}
	assertClaimsSurvived(t, parsed)

	// a token signed by a different key must be rejected
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate other RSA key: %v", err)
	}
	otherPublicKeyDER, err := x509.MarshalPKIXPublicKey(&otherKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal other RSA public key: %v", err)
	}
	otherPublicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: otherPublicKeyDER,
	}))
	if _, err := ValidateJWTWithRSAPublicKey(tokenString, otherPublicKeyPEM); err == nil {
		t.Error("ValidateJWTWithRSAPublicKey() with wrong key expected error, got nil")
	}
}
