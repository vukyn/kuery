package cryp

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashArgon2idRoundTrip(t *testing.T) {
	hash := HashArgon2id("s3cret-password")

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
	if !CompareArgon2id("s3cret-password", hash) {
		t.Error("expected matching password to verify")
	}
	if CompareArgon2id("wrong-password", hash) {
		t.Error("expected wrong password to fail verification")
	}
}

func TestHashArgon2idUniqueSalt(t *testing.T) {
	first := HashArgon2id("same-password")
	second := HashArgon2id("same-password")
	if first == second {
		t.Error("expected two hashes of the same password to differ (random salt)")
	}
}

func TestCompareArgon2idMalformed(t *testing.T) {
	valid := HashArgon2id("hello")

	tests := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"not a hash", "hello world"},
		{"wrong algorithm", strings.Replace(valid, "argon2id", "argon2i", 1)},
		{"wrong version", strings.Replace(valid, "v=19", "v=18", 1)},
		{"missing params", "$argon2id$v=19$$salt$key"},
		{"non-numeric params", "$argon2id$v=19$m=abc,t=2,p=1$c2FsdHNhbHRzYWx0c2E$a2V5a2V5"},
		{"zero time", "$argon2id$v=19$m=19456,t=0,p=1$c2FsdHNhbHRzYWx0c2E$a2V5a2V5"},
		{"zero parallelism", "$argon2id$v=19$m=19456,t=2,p=0$c2FsdHNhbHRzYWx0c2E$a2V5a2V5"},
		{"invalid salt base64", strings.Replace(valid, "$m=19456,t=2,p=1$", "$m=19456,t=2,p=1$!!!invalid$", 1)},
		{"truncated", valid[:len(valid)/2]},
		{"too many segments", valid + "$extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CompareArgon2id("hello", tt.hash) {
				t.Errorf("expected malformed hash to fail verification: %s", tt.hash)
			}
		})
	}
}

func TestCompareArgon2idParsedParams(t *testing.T) {
	// Hand-build a PHC string with non-default parameters to assert that
	// verification recomputes with the parsed values, not the hardcoded ones.
	password := "hello"
	salt := []byte("0123456789abcdef")
	var memory uint32 = 8192
	var time uint32 = 1
	var parallelism uint8 = 1

	key := argon2.IDKey([]byte(password), salt, time, memory, parallelism, 32)
	hash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory, time, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)

	if !CompareArgon2id(password, hash) {
		t.Error("expected hash with non-default params to verify using parsed params")
	}
	if CompareArgon2id("wrong-password", hash) {
		t.Error("expected wrong password to fail verification")
	}
}

func TestVerifyPassword(t *testing.T) {
	argonHash := HashArgon2id("hello")
	bcryptHash := HashBcrypt("hello", 0)

	tests := []struct {
		name            string
		password        string
		hash            string
		wantOk          bool
		wantNeedsRehash bool
	}{
		{"argon2id match", "hello", argonHash, true, false},
		{"argon2id mismatch", "wrong", argonHash, false, false},
		{"bcrypt match", "hello", bcryptHash, true, true},
		{"bcrypt mismatch", "wrong", bcryptHash, false, false},
		{"unknown prefix", "hello", "$unknown$abcdef", false, false},
		{"empty hash", "hello", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, needsRehash := VerifyPassword(tt.password, tt.hash)
			if ok != tt.wantOk || needsRehash != tt.wantNeedsRehash {
				t.Errorf("VerifyPassword() = (%v, %v), want (%v, %v)", ok, needsRehash, tt.wantOk, tt.wantNeedsRehash)
			}
		})
	}
}
