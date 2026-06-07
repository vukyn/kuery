package cryp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id hashing parameters following the OWASP password storage recommendation.
const (
	argon2idMemory      uint32 = 19456 // KiB
	argon2idTime        uint32 = 2
	argon2idParallelism uint8  = 1
	argon2idSaltLength  uint32 = 16
	argon2idKeyLength   uint32 = 32
)

// HashArgon2id hashes password with the Argon2id algorithm using OWASP
// recommended parameters (memory 19456 KiB, time 2, parallelism 1) and a
// random 16-byte salt, returning a standard PHC-formatted string
//
// Example:
//
//	HashArgon2id("hello") => "$argon2id$v=19$m=19456,t=2,p=1$nibIRtBLEz1srGbF1FbDsg$nU/AC9NwBHQHvkU1+yhz9NF14SnXn7rwHTKKZjj7CkE"
func HashArgon2id(password string) string {
	salt := make([]byte, argon2idSaltLength)
	_, _ = rand.Read(salt)

	key := argon2.IDKey([]byte(password), salt, argon2idTime, argon2idMemory, argon2idParallelism, argon2idKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2idMemory, argon2idTime, argon2idParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// CompareArgon2id compares password with an Argon2id PHC-formatted hash,
// return true if password matches the hash, otherwise false.
// The cost parameters are read from the hash itself, so hashes created
// with different parameters keep verifying after a parameter bump.
// Malformed hashes return false (never panic)
//
// Example:
//
//	CompareArgon2id("hello", "$argon2id$v=19$m=19456,t=2,p=1$nibIRtBLEz1srGbF1FbDsg$nU/AC9NwBHQHvkU1+yhz9NF14SnXn7rwHTKKZjj7CkE") => true
func CompareArgon2id(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}

	var memory, time uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallelism); err != nil {
		return false
	}
	if time == 0 || parallelism == 0 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return false
	}

	computed := argon2.IDKey([]byte(password), salt, time, memory, parallelism, uint32(len(key)))
	return subtle.ConstantTimeCompare(computed, key) == 1
}

// VerifyPassword verifies password against a stored hash of any supported
// scheme, return ok if the password matches and needsRehash if the stored
// hash uses a legacy scheme and should be upgraded to Argon2id.
// Unknown hash formats return (false, false)
//
// Example:
//
//	VerifyPassword("hello", "$argon2id$v=19$...") => (true, false)
//	VerifyPassword("hello", "$2a$10$...")         => (true, true)
func VerifyPassword(password, hash string) (ok bool, needsRehash bool) {
	switch {
	case strings.HasPrefix(hash, "$argon2id$"):
		return CompareArgon2id(password, hash), false
	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
		ok := CompareBcrypt(password, hash)
		return ok, ok
	default:
		return false, false
	}
}
