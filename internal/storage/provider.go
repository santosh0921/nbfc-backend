// Package storage provides encrypted-at-rest storage for KYC document
// bytes — replacing the plaintext base64 TEXT column
// (internal/models.Document.DataBase64) that previously held every
// uploaded Aadhaar/PAN/bill/statement scan directly in the primary
// database with no encryption at all.
//
// The active provider (local_encrypted.go) writes AES-256-GCM-encrypted
// files to disk rather than the database — a real encryption-at-rest
// improvement achievable without any cloud credentials this environment
// doesn't have. The Provider interface is deliberately narrow so a real
// object-storage backend (S3/GCS, with genuine signed/temporary download
// URLs — not achievable against a local filesystem) can be swapped in
// later without touching internal/documents' handler logic.
package storage

import "github.com/google/uuid"

type Provider interface {
	Name() string
	// Store encrypts and persists data, returning an opaque reference the
	// caller stores in Document.StorageReference — never a raw filesystem
	// path a client could manipulate.
	Store(customerID, docID uuid.UUID, data []byte) (reference string, err error)
	Retrieve(reference string) ([]byte, error)
	Delete(reference string) error
}

var active Provider

// Initialize wires up the configured provider. Called once from
// cmd/api/main.go. Prefers S3 (survives deploys/instance replacement —
// see s3.go's doc comment for why local disk silently lost every document
// on the first redeploy after upload) whenever DOCUMENT_STORAGE_BUCKET is
// set; falls back to local-disk storage otherwise (fine for local dev,
// never for production — see config.ValidateProductionSafety). Falls back
// further to a same-behavior-as-before failure mode (nil provider,
// document upload endpoints return a clear 503) if DOCUMENT_ENCRYPTION_KEY
// is missing entirely — never silently stores unencrypted.
func Initialize() {
	if p := newS3Provider(); p != nil {
		active = p
		return
	}
	if p := newLocalEncryptedProvider(); p != nil {
		active = p
	}
}

func Active() Provider {
	return active
}

// SetActiveForTesting overrides the active provider for this package's
// own tests. Never called from production code.
func SetActiveForTesting(p Provider) {
	active = p
}
