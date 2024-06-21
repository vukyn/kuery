package cryp

import (
	"crypto/md5"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// Hash content with MD5 algorithm
func HashMD5(content string) string {
	h := md5.New()
	h.Write([]byte(content)) //nolint
	cipher := hex.EncodeToString(h.Sum(nil))
	return cipher
}

// Hash uuid
func HashUUID() string {
	return string(HashMD5(UUID()))
}

// Hash ulid
func HashULID() string {
	return string(HashMD5(ULID()))
}

// Hash bcrypt, pass cost 0 will use default cost
func HashBcrypt(content string, cost int) string {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	hashed, _ := bcrypt.GenerateFromPassword([]byte(content), cost)
	return string(hashed)
}
