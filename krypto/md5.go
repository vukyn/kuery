package krypto

import (
	"crypto/md5"
	"encoding/hex"
)

// Md5Encrypt ...
func Md5Encrypt(content []byte) []byte {
	h := md5.New()
	h.Write(content) //nolint
	cipher := hex.EncodeToString(h.Sum(nil))
	return []byte(cipher)
}
