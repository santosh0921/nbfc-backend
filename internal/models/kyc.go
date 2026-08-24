package models

import (
	"time"

	"github.com/google/uuid"
)

// KYC lifecycle (Phase 3) — the backend-authoritative state machine. No
// customer-facing endpoint can set KYCStatusVerified directly; it is only
// ever set by internal/kyc.Service after checking every underlying
// verification (PAN/Aadhaar/Selfie provider results) plus an admin's
// explicit approval (internal/admin, KYC_APPROVE permission) — mirroring
// how CustomerKYCReview already worked, just formalized into a real state
// machine other services (loan submission, credit decisions) can gate on.
const (
	KYCStatusNotStarted         = "NOT_STARTED"
	KYCStatusInitiated          = "INITIATED"           // profile/address on file, no documents yet
	KYCStatusDocumentsPending   = "DOCUMENTS_PENDING"    // some but not all required documents submitted
	KYCStatusVerificationPending = "VERIFICATION_PENDING" // all documents submitted, provider/admin verification not yet complete
	KYCStatusVerified           = "VERIFIED"
	KYCStatusRejected           = "REJECTED"
	KYCStatusExpired            = "EXPIRED"
	KYCStatusRequiresReview     = "REQUIRES_REVIEW"
)

// KYCLevelBasic today — the only level this app currently verifies
// (profile+address+PAN+Aadhaar+selfie). VerificationLevel exists as a
// field, not a hardcoded assumption, so a future "enhanced" tier (e.g.
// video KYC + income proof) can be added without a schema change.
const KYCLevelBasic = "BASIC"

// CustomerKYC is the single authoritative row per customer for the state
// machine above. Recomputed by internal/kyc.Service.Recompute from the
// underlying CustomerProfile/CustomerAddress/CustomerPAN/CustomerAadhaar/
// CustomerSelfie/CustomerKYCReview rows — it does not duplicate their
// data, only the derived state and the audit-relevant who/when of the
// last transition.
type CustomerKYC struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"kycId"`

	CustomerID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"customerId"`

	Status            string `gorm:"size:30;not null;default:'NOT_STARTED'" json:"status"`
	VerificationLevel string `gorm:"size:20;not null;default:'BASIC'" json:"verificationLevel"`

	// Provider/ProviderReference describe whichever check most recently
	// drove a status change (e.g. "SUREPASS" for a PAN result, "ADMIN" for
	// a manual review decision) — informational, not a single source of
	// truth (the underlying per-document rows remain that).
	Provider         string `json:"provider,omitempty"`
	ProviderReference string `json:"providerReference,omitempty"`

	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	VerifiedBy string     `json:"verifiedBy,omitempty"` // admin username, or "system" for a fully-automated pass

	FailureReason string `json:"failureReason,omitempty"`
	ReviewReason  string `json:"reviewReason,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
