package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	SALT = "ZAjrbWED"
)

// deriveKey derives a 32-byte AES key from the secret using HKDF or SHA-256
func deriveKey(secret, ctxInfo string) ([]byte, error) {
	if secret == "" {
		return nil, errors.New("secret is required")
	}
	if ctxInfo == "" {
		return nil, errors.New("ctxInfo is required")
	}

	secretBytes := []byte(secret)

	// If both salt and ctxInfo are provided, use HKDF
	if ctxInfo != "" {
		saltBytes := []byte(SALT)
		info := []byte(ctxInfo)

		// Use HKDF with SHA-256 to derive a 32-byte key
		hash := sha256.New
		hkdf := hkdf.New(hash, secretBytes, saltBytes, info)

		key := make([]byte, 32) // AES-256 key size
		if _, err := io.ReadFull(hkdf, key); err != nil {
			return nil, errors.New("failed to derive key using HKDF: " + err.Error())
		}

		return key, nil
	}

	// Otherwise, use SHA-256 hash of the secret
	hash := sha256.Sum256(secretBytes)
	return hash[:], nil
}

// Encrypt encrypts the plaintext using AES-GCM
func Encrypt(text, secret, ctxInfo string) (string, error) {
	if text == "" {
		return "", errors.New("text is required")
	}

	// Derive the encryption key
	key, err := deriveKey(secret, ctxInfo)
	if err != nil {
		return "", err
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.New("failed to create AES cipher: " + err.Error())
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errors.New("failed to create GCM: " + err.Error())
	}

	// Generate random nonce (12 bytes for GCM)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errors.New("failed to generate nonce: " + err.Error())
	}

	// Encrypt the plaintext
	plaintext := []byte(text)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Encode as Base64
	encryptedText := base64.StdEncoding.EncodeToString(ciphertext)

	return encryptedText, nil
}

// Decrypt decrypts the Base64 encoded ciphertext using AES-GCM
func Decrypt(text, secret, ctxInfo string) (string, error) {
	if text == "" {
		return "", errors.New("text is required")
	}

	// Decode Base64 input
	ciphertext, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", errors.New("failed to decode Base64: " + err.Error())
	}

	// Derive the decryption key (same as encryption)
	key, err := deriveKey(secret, ctxInfo)
	if err != nil {
		return "", err
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.New("failed to create AES cipher: " + err.Error())
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errors.New("failed to create GCM: " + err.Error())
	}

	// Check if ciphertext is long enough to contain nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the ciphertext
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("failed to decrypt: " + err.Error())
	}

	return string(plaintext), nil
}
