package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	pkgClaims "github.com/vukyn/kuery/claims"

	"github.com/golang-jwt/jwt/v5"
)

// normalizePEM converts literal "\n" escape sequences into real newlines so a
// PEM stored as a single-line env var / secret (e.g. a fly.io secret, which
// does not expand backslash escapes the way godotenv does) still parses. A
// well-formed PEM never contains a backslash, so this is a no-op otherwise.
func normalizePEM(key string) string {
	if strings.Contains(key, "\\n") {
		return strings.ReplaceAll(key, "\\n", "\n")
	}
	return key
}

// GenerateJWT generates a JWT token using HMAC (HS256)
func GenerateJWT(secretKey string, expireIn int, userID, email string) (string, pkgClaims.Claims, error) {
	claims := pkgClaims.NewClaims(userID, email, int64(expireIn))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims.MapClaims)
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", pkgClaims.Claims{}, err
	}
	return tokenString, claims, nil
}

// GenerateJWTFromClaims generates a JWT token from prebuilt claims using HMAC (HS256)
func GenerateJWTFromClaims(secretKey string, claims pkgClaims.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims.MapClaims)
	return token.SignedString([]byte(secretKey))
}

// ValidateJWT validates a JWT token using HMAC (HS256)
func ValidateJWT(tokenString, secretKey string) (pkgClaims.Claims, error) {
	claims := pkgClaims.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims.MapClaims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, errors.New("invalid token")
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return pkgClaims.Claims{}, err
	}
	if !token.Valid {
		return pkgClaims.Claims{}, errors.New("invalid token")
	}
	return claims, nil
}

// parseRSAPrivateKey parses RSA private key from PEM string
func parseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(privateKeyPEM)))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key (PKCS1: %v; PKCS8: %v)", err, err2)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("key is not an RSA private key")
		}
		return rsaKey, nil
	}

	return privateKey, nil
}

// parseRSAPublicKey parses RSA public key from PEM string
func parseRSAPublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(publicKeyPEM)))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try PKCS1 format
		pub, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse public key (PKIX: %v; PKCS1: %v)", err, err2)
		}
		return pub, nil
	}

	rsaKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}

	return rsaKey, nil
}

// GenerateJWTWithRSAPrivateKey generates a JWT token using RSA private key (RS256) - used for access tokens
func GenerateJWTWithRSAPrivateKey(privateKeyPEM string, expireIn int, userID, email string) (string, pkgClaims.Claims, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", pkgClaims.Claims{}, err
	}

	claims := pkgClaims.NewClaims(userID, email, int64(expireIn))
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims.MapClaims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", pkgClaims.Claims{}, err
	}
	return tokenString, claims, nil
}

// GenerateJWTWithRSAPrivateKeyFromClaims generates a JWT token from prebuilt claims using RSA private key (RS256)
func GenerateJWTWithRSAPrivateKeyFromClaims(privateKeyPEM string, claims pkgClaims.Claims) (string, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims.MapClaims)
	return token.SignedString(privateKey)
}

// ValidateJWTWithRSAPublicKey validates a JWT token using RSA public key (RS256) - used for access tokens
func ValidateJWTWithRSAPublicKey(tokenString, publicKeyPEM string) (pkgClaims.Claims, error) {
	publicKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return pkgClaims.Claims{}, err
	}

	claims := pkgClaims.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims.MapClaims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Name {
			return nil, errors.New("invalid token signing method")
		}
		return publicKey, nil
	})
	if err != nil {
		return pkgClaims.Claims{}, err
	}
	if !token.Valid {
		return pkgClaims.Claims{}, errors.New("invalid token")
	}
	return claims, nil
}
