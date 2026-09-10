package cryp

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"time"

	uuid "github.com/google/uuid"
	ulid "github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

// UUID generate a UUID
//
// Example:
//
//	UUID() => "f47ac10b-58cc-4372-a567-0e02b2c3d479"
func UUID() string {
	return uuid.New().String()
}

// ULID generates a ULID whose entropy comes from crypto/rand.
//
// ⚠️ It deliberately does NOT use ulid.Make(). That helper draws from
// ulid.DefaultEntropy(), which is a process-wide math/rand seeded once from
// time.Now().UnixNano() and then advanced monotonically. The 48-bit prefix of
// every ULID is already a plain millisecond timestamp, so an observer who holds
// a handful of ULIDs from a process can recover that seed offline and then
// compute every ULID that process will ever mint.
//
// That is harmless when a ULID is only a database row id, which is most of its
// use — but callers across this platform also use it as a CREDENTIAL: an OAuth
// authorization code, a session id, a JWT jti, an object-storage key. Those must
// be unguessable, and one predictable generator behind all of them is a single
// point of failure. Drawing from crypto/rand costs nothing measurable and
// removes the distinction entirely, so the safe source is the only source.
//
// Sort order is preserved: the timestamp prefix is unchanged, so ULIDs still
// sort lexicographically by creation time. What is lost is ulid.Make's
// monotonic guarantee WITHIN a single millisecond — two ULIDs minted in the
// same millisecond now order randomly relative to each other. No caller here
// depends on sub-millisecond ordering.
//
// Example:
//
//	ULID() => "01D7Z9Z1ZQ0QZQZQZQZQZQZQZQ"
func ULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Hash content with MD5 algorithm
//
// Warning: MD5 is cryptographically broken. Use only for
// non-security checksums; prefer HashSHA256 or HashBcrypt for
// anything security-sensitive.
//
// Example:
//
//	HashMD5("hello") => "5d41402abc4b2a76b9719d911017c592"
func HashMD5(content string) string {
	h := md5.New()
	h.Write([]byte(content)) //nolint
	cipher := hex.EncodeToString(h.Sum(nil))
	return cipher
}

// Hash content with SHA256 algorithm
//
// Example:
//
//	HashSHA256("hello") => "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"
func HashSHA256(content string) string {
	h := sha256.New()
	h.Write([]byte(content)) //nolint
	cipher := hex.EncodeToString(h.Sum(nil))
	return cipher
}

// Hash content with SHA512 algorithm
//
// Example:
//
//	HashSHA512("hello") => "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"
func HashSHA512(content string) string {
	h := sha512.New()
	h.Write([]byte(content)) //nolint
	cipher := hex.EncodeToString(h.Sum(nil))
	return cipher
}

// Hash bcrypt, pass cost 0 will use default cost
//
// Example:
//
//	HashBcrypt("hello", 0) => "$2a$10$Smi2Acrukt0SgFp4PfdSTOifok7p9gxDHupjsR6oga5Sa5ONsqwsq"
func HashBcrypt(content string, cost int) string {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	hashed, _ := bcrypt.GenerateFromPassword([]byte(content), cost)
	return string(hashed)
}

// CompareBcrypt compare content with hashed content,
// return true if content is the same as hashed content, otherwise false
//
// Example:
//
//	CompareBcrypt("hello", "$2a$10$Smi2Acrukt0SgFp4PfdSTOifok7p9gxDHupjsR6oga5Sa5ONsqwsq") => true
func CompareBcrypt(content, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(content)) == nil
}
