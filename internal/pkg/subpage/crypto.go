// Package subpage contains the domain logic for serving Remnawave
// subscription pages: panel API access, config caching, and uuid
// encryption.
package subpage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
)

func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// EncryptUUID encrypts uuid with AES-256-GCM, key derived from secret via
// SHA-256, and returns base64url(iv || tag || ciphertext). Mirrors
// crypt-utils.ts encryptUuid.
func EncryptUUID(uuid, secret string) (string, error) {
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	// Seal appends the tag to the ciphertext; we want iv || tag || ciphertext
	// to match the Node implementation, so split them back apart.
	sealed := gcm.Seal(nil, iv, []byte(uuid), nil)
	tagSize := gcm.Overhead()
	ciphertext := sealed[:len(sealed)-tagSize]
	tag := sealed[len(sealed)-tagSize:]

	out := make([]byte, 0, len(iv)+len(tag)+len(ciphertext))
	out = append(out, iv...)
	out = append(out, tag...)
	out = append(out, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// DecryptUUID reverses EncryptUUID. Returns ("", false) on any failure
// (wrong key, tampered data, malformed input) matching the Node behavior
// of returning null rather than throwing.
func DecryptUUID(data, secret string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return "", false
	}
	if len(raw) < 12+16 {
		return "", false
	}
	iv := raw[:12]
	tag := raw[12:28]
	ciphertext := raw[28:]

	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false
	}
	sealed := append(append([]byte{}, ciphertext...), tag...)
	plaintext, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return "", false
	}
	return string(plaintext), true
}
