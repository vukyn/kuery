package krypto

import (
	"github.com/google/uuid"
)

// Hashed uuid token
func HashedToken() string {
	return string(Md5Encrypt([]byte(uuid.New().String())))
}
