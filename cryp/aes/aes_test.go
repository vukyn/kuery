package aes

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		secret  string
		ctxInfo string
	}{
		{
			name:    "simple ascii",
			text:    "hello world",
			secret:  "super-secret-key",
			ctxInfo: "user:42",
		},
		{
			name:    "unicode payload",
			text:    "Xin chào thế giới 🌏",
			secret:  "another-secret",
			ctxInfo: "session:abc",
		},
		{
			name:    "long payload",
			text:    "The quick brown fox jumps over the lazy dog. " + "repeated many times over and over again to exceed a single block.",
			secret:  "long-secret-value-1234567890",
			ctxInfo: "ctx:long",
		},
		{
			name:    "single char",
			text:    "x",
			secret:  "s",
			ctxInfo: "c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.text, tt.secret, tt.ctxInfo)
			if err != nil {
				t.Fatalf("Encrypt() unexpected error: %v", err)
			}
			if encrypted == tt.text {
				t.Fatalf("Encrypt() returned plaintext unchanged")
			}

			decrypted, err := Decrypt(encrypted, tt.secret, tt.ctxInfo)
			if err != nil {
				t.Fatalf("Decrypt() unexpected error: %v", err)
			}
			if decrypted != tt.text {
				t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, tt.text)
			}
		})
	}
}

func TestEncryptNonDeterministicNonce(t *testing.T) {
	// GCM uses a random nonce, so encrypting the same input twice must yield
	// different ciphertext while both still decrypt back to the original.
	first, err := Encrypt("same input", "secret", "ctx")
	if err != nil {
		t.Fatalf("Encrypt() unexpected error: %v", err)
	}
	second, err := Encrypt("same input", "secret", "ctx")
	if err != nil {
		t.Fatalf("Encrypt() unexpected error: %v", err)
	}
	if first == second {
		t.Fatalf("expected different ciphertext for repeated encryption (random nonce)")
	}
}

func TestEncryptValidation(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		secret  string
		ctxInfo string
	}{
		{name: "empty text", text: "", secret: "s", ctxInfo: "c"},
		{name: "empty secret", text: "t", secret: "", ctxInfo: "c"},
		{name: "empty ctxInfo", text: "t", secret: "s", ctxInfo: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Encrypt(tt.text, tt.secret, tt.ctxInfo); err == nil {
				t.Fatalf("Encrypt() expected error, got nil")
			}
		})
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	encrypted, err := Encrypt("top secret", "correct-secret", "ctx:1")
	if err != nil {
		t.Fatalf("Encrypt() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		secret  string
		ctxInfo string
	}{
		{name: "wrong secret", secret: "wrong-secret", ctxInfo: "ctx:1"},
		{name: "wrong ctxInfo", secret: "correct-secret", ctxInfo: "ctx:2"},
		{name: "both wrong", secret: "wrong-secret", ctxInfo: "ctx:2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decrypt(encrypted, tt.secret, tt.ctxInfo); err == nil {
				t.Fatalf("Decrypt() with wrong key expected error, got nil")
			}
		})
	}
}

func TestDecryptMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "empty text", text: ""},
		{name: "invalid base64", text: "not-valid-base64!!!"},
		{name: "too short ciphertext", text: "YWJj"}, // "abc" base64, shorter than nonce
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decrypt(tt.text, "secret", "ctx"); err == nil {
				t.Fatalf("Decrypt() expected error for malformed input, got nil")
			}
		})
	}
}

func TestDeriveKey(t *testing.T) {
	t.Run("deterministic and correct length", func(t *testing.T) {
		key, err := deriveKey("secret", "ctx")
		if err != nil {
			t.Fatalf("deriveKey() unexpected error: %v", err)
		}
		if len(key) != 32 {
			t.Fatalf("deriveKey() length = %d, want 32", len(key))
		}
		again, err := deriveKey("secret", "ctx")
		if err != nil {
			t.Fatalf("deriveKey() unexpected error: %v", err)
		}
		if string(key) != string(again) {
			t.Fatalf("deriveKey() not deterministic for identical inputs")
		}
	})

	t.Run("different inputs derive different keys", func(t *testing.T) {
		base, _ := deriveKey("secret", "ctx")
		diffSecret, _ := deriveKey("secret2", "ctx")
		diffCtx, _ := deriveKey("secret", "ctx2")

		if string(base) == string(diffSecret) {
			t.Fatalf("different secret produced identical key")
		}
		if string(base) == string(diffCtx) {
			t.Fatalf("different ctxInfo produced identical key")
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		if _, err := deriveKey("", "ctx"); err == nil {
			t.Fatalf("deriveKey() expected error for empty secret")
		}
		if _, err := deriveKey("secret", ""); err == nil {
			t.Fatalf("deriveKey() expected error for empty ctxInfo")
		}
	})
}
