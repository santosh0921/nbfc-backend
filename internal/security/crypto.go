// Package security provides symmetric encryption helpers for sensitive
// fields (PAN numbers) that must be stored at rest but occasionally need
// to be recovered server-side. Aadhaar is handled separately (hash + mask
// only, never stored/recovered — see models.CustomerAadhaar) since it's
// never required in cleartext again after verification.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"sync"
)

var (
	keyOnce sync.Once
	aesKey  []byte
	keyErr  error
)

// loadKey reads PAN_ENCRYPTION_KEY (base64-encoded 32-byte AES-256 key)
// from the environment once and caches it.
func loadKey() ([]byte, error) {
	keyOnce.Do(func() {
		raw := os.Getenv("PAN_ENCRYPTION_KEY")
		if raw == "" {
			keyErr = errors.New("security: PAN_ENCRYPTION_KEY is not set")
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			keyErr = errors.New("security: PAN_ENCRYPTION_KEY is not valid base64: " + err.Error())
			return
		}
		if len(decoded) != 32 {
			keyErr = errors.New("security: PAN_ENCRYPTION_KEY must decode to exactly 32 bytes (AES-256)")
			return
		}
		aesKey = decoded
	})
	return aesKey, keyErr
}

// Encrypt returns a base64-encoded AES-256-GCM ciphertext (nonce prepended)
// for the given plaintext.
func Encrypt(plaintext string) (string, error) {
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt. Only ever call this server-side for the rare
// operational case that needs the raw value (e.g. re-submitting to a
// verification provider) — never to build an API response.
func Decrypt(ciphertext string) (string, error) {
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("security: ciphertext too short")
	}
	nonce, sealed := raw[:nonceSize], raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// MaskPAN returns a display-safe masked form of a PAN, e.g. "******1234A"
// style (last 4 chars visible), derived only from the plaintext last-4
// digits so callers never need to decrypt just to render a masked value.
func MaskPAN(last4 string) string {
	if last4 == "" {
		return "**********"
	}
	return "******" + last4
}
