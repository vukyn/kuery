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
		WithPerms([]string{"user:read", "role:read"}).
		WithRoles([]string{"member"}).
		WithIsAdmin(true)
}

func assertClaimsSurvived(t *testing.T, parsed pkgClaims.Claims) {
	t.Helper()
	if got := parsed.GetUserID(); got != "user-1" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-1")
	}
	if got := parsed.GetEmail(); got != "user@example.com" {
		t.Errorf("GetEmail() = %q, want %q", got, "user@example.com")
	}
	if got, want := parsed.GetPerms(), []string{"user:read", "role:read"}; !slices.Equal(got, want) {
		t.Errorf("GetPerms() = %v, want %v", got, want)
	}
	if got, want := parsed.GetRoles(), []string{"member"}; !slices.Equal(got, want) {
		t.Errorf("GetRoles() = %v, want %v", got, want)
	}
	if !parsed.GetIsAdmin() {
		t.Error("GetIsAdmin() = false, want true")
	}
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
