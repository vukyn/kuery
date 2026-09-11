package cryp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"runtime"
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

// TestCompareArgon2idRejectsUnboundedCostParameters is the correctness half of
// the bound: a hash carrying an out-of-range cost parameter must not verify.
//
// ⚠️ On its own this proves nothing. An out-of-range hash also fails to verify
// for the boring reason — the key does not match — so this test passes with the
// bound removed. The bound is load-bearing only in the two tests below, which
// assert the WORK IS NOT DONE. Keeping them apart is deliberate: a single
// "returns false" test here would have looked like coverage and been worthless.
func TestCompareArgon2idRejectsUnboundedCostParameters(t *testing.T) {
	cases := []struct {
		name   string
		params string
	}{
		{"memory at uint32 max", "m=4294967295,t=2,p=1"},
		{"memory just over the bound", fmt.Sprintf("m=%d,t=2,p=1", maxArgon2idMemory+1)},
		{"memory zero", "m=0,t=2,p=1"},
		{"time over the bound", fmt.Sprintf("m=19456,t=%d,p=1", maxArgon2idTime+1)},
		{"time zero", "m=19456,t=0,p=1"},
		{"parallelism zero", "m=19456,t=2,p=0"},
	}

	salt := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef"))
	key := base64.RawStdEncoding.EncodeToString(make([]byte, 32))

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			hash := fmt.Sprintf("$argon2id$v=%d$%s$%s$%s", argon2.Version, testCase.params, salt, key)
			if CompareArgon2id("any-password", hash) {
				t.Fatal("an out-of-range cost parameter must not verify")
			}
		})
	}
}

// TestCompareArgon2idDoesNotRunTheKDFForAnOversizedMemoryParameter is the real
// guard for the resource-exhaustion path. CompareArgon2id takes its memory
// parameter from the hash STRING, so that value decides the allocation size —
// m=4294967295 asks argon2.IDKey for roughly 4 TB.
//
// The assertion is the allocation, not the result: with the bound in place the
// call returns before argon2.IDKey is entered and allocates essentially
// nothing. It uses a value just over the bound rather than the uint32 maximum
// so that removing the bound produces a clean 1 GiB test FAILURE instead of
// taking the machine down — which is the hazard itself, not a useful demo.
//
// Mutation: delete the memory bound and the observed allocation jumps from
// kilobytes to ~1 GiB and this fails.
func TestCompareArgon2idDoesNotRunTheKDFForAnOversizedMemoryParameter(t *testing.T) {
	salt := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef"))
	key := base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	hash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=2,p=1$%s$%s", argon2.Version, maxArgon2idMemory+1, salt, key)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	if CompareArgon2id("any-password", hash) {
		t.Fatal("an out-of-range memory parameter must not verify")
	}

	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc

	// argon2 at the requested size would allocate ~1 GiB in one go. The ceiling
	// sits far above ordinary test noise and far below that.
	const ceiling = 64 << 20
	if allocated > ceiling {
		t.Fatalf("CompareArgon2id allocated %d bytes for a hash asking for %d KiB — the cost parameter is being trusted before it is checked",
			allocated, uint64(maxArgon2idMemory)+1)
	}
}

// TestCompareArgon2idDoesNotRunTheKDFForAnOversizedTimeParameter is the same
// guard for the iteration count, asserted differently because an oversized t is
// cheap enough that allocation cannot distinguish it.
//
// The hash here is GENUINE — produced by running argon2 with an out-of-range t
// over the real password — so if the bound were removed, CompareArgon2id would
// recompute the same key and return TRUE. The bound is therefore the only
// reason this returns false, and the test cannot pass for the boring reason.
//
// Mutation: delete the time bound and this fails.
func TestCompareArgon2idDoesNotRunTheKDFForAnOversizedTimeParameter(t *testing.T) {
	const password = "s3cret-password"
	oversizedTime := maxArgon2idTime + 1

	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte(password), salt, oversizedTime, argon2idMemory, argon2idParallelism, argon2idKeyLength)
	hash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2idMemory, oversizedTime, argon2idParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)

	// Sanity: without the bound this hash WOULD verify, which is what makes the
	// assertion below meaningful rather than incidental.
	if !bytes.Equal(key, argon2.IDKey([]byte(password), salt, oversizedTime, argon2idMemory, argon2idParallelism, argon2idKeyLength)) {
		t.Fatal("argon2 is not deterministic here — this test cannot mean what it claims")
	}

	if CompareArgon2id(password, hash) {
		t.Fatal("a hash with an out-of-range time parameter must be rejected even when the password is correct")
	}
}

// TestCompareArgon2idBoundIsAnUpperLimitNotAWhitelist is the other direction:
// the bounds must not reject ordinary hashes, including this library's own.
func TestCompareArgon2idBoundIsAnUpperLimitNotAWhitelist(t *testing.T) {
	hash := HashArgon2id("s3cret-password")
	if !CompareArgon2id("s3cret-password", hash) {
		t.Fatal("a hash produced by HashArgon2id must still verify — the bound must not reject ordinary parameters")
	}
	if CompareArgon2id("wrong-password", hash) {
		t.Fatal("a wrong password must not verify")
	}
	// The shipped parameters must sit inside the bounds, or every login breaks.
	if argon2idMemory > maxArgon2idMemory {
		t.Fatalf("argon2idMemory (%d) exceeds the bound (%d) — the library rejects its own hashes", argon2idMemory, maxArgon2idMemory)
	}
	if argon2idTime > maxArgon2idTime {
		t.Fatalf("argon2idTime (%d) exceeds the bound (%d) — the library rejects its own hashes", argon2idTime, maxArgon2idTime)
	}
}
