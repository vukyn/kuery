package cryp

import (
	"crypto/md5"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// Hash content with MD5 algorithm
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

// Hash uuid
//
// Example:
//
//	UUID() => "f47ac10b-58cc-4372-a567-0e02b2c3d479"
func HashUUID() string {
	return string(HashMD5(UUID()))
}

// Hash ulid
//
// Example:
//
//	ULID() => "01D7Z9Z1ZQ0QZQZQZQZQZQZQZQ"
func HashULID() string {
	return string(HashMD5(ULID()))
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
