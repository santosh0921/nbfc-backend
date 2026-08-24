package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// localEncryptedProvider stores each document as an AES-256-GCM-encrypted
// file under basePath, keyed by a random reference (not the customer/doc
// UUIDs directly, so a leaked reference alone doesn't reveal whose
// document it is or which document type). Uses its own
// DOCUMENT_ENCRYPTION_KEY, deliberately separate from PAN_ENCRYPTION_KEY
// (internal/security) — a document-storage key compromise shouldn't also
// expose PAN numbers, and vice versa.
type localEncryptedProvider struct {
	basePath string
	key      []byte
}

func newLocalEncryptedProvider() *localEncryptedProvider {
	raw := strings.TrimSpace(os.Getenv("DOCUMENT_ENCRYPTION_KEY"))
	if raw == "" {
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil
	}
	basePath := strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_PATH"))
	if basePath == "" {
		basePath = "./storage/documents"
	}
	if err := os.MkdirAll(basePath, 0o700); err != nil {
		return nil
	}
	return &localEncryptedProvider{basePath: basePath, key: key}
}

func (p *localEncryptedProvider) Name() string { return "local_encrypted" }

func (p *localEncryptedProvider) Store(customerID, docID uuid.UUID, data []byte) (string, error) {
	block, err := aes.NewCipher(p.key)
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
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// The reference is a fresh random ID, not docID itself — the on-disk
	// filename alone should not be guessable from (or reveal) the
	// document's database ID.
	reference := uuid.NewString()
	path := p.pathFor(reference)
	if err := os.WriteFile(path, ciphertext, 0o600); err != nil {
		return "", err
	}
	return reference, nil
}

func (p *localEncryptedProvider) Retrieve(reference string) ([]byte, error) {
	path, err := p.safePathFor(reference)
	if err != nil {
		return nil, err
	}
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("storage: corrupt or truncated document")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, sealed, nil)
}

func (p *localEncryptedProvider) Delete(reference string) error {
	path, err := p.safePathFor(reference)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (p *localEncryptedProvider) pathFor(reference string) string {
	return filepath.Join(p.basePath, reference+".enc")
}

// safePathFor rejects any reference that isn't a plain UUID before
// building a filesystem path from it — reference values ultimately come
// from a database column, but this keeps Retrieve/Delete safe against
// path traversal even if that ever changes.
func (p *localEncryptedProvider) safePathFor(reference string) (string, error) {
	if _, err := uuid.Parse(reference); err != nil {
		return "", errors.New("storage: invalid document reference")
	}
	return p.pathFor(reference), nil
}
