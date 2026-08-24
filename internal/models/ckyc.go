package models

import (
	"time"

	"github.com/google/uuid"
)

// CKYC/CERSAI request types and statuses (Phase 3, item J). No real CKYC
// registry credentials/certification are configured in this environment —
// see internal/ckyc's doc comment for what that means for production.
const (
	CKYCRequestTypeSearch   = "SEARCH"
	CKYCRequestTypeRetrieve = "RETRIEVE"
	CKYCRequestTypeCreate   = "CREATE"
	CKYCRequestTypeUpdate   = "UPDATE"
)

const (
	CKYCStatusPending = "PENDING"
	CKYCStatusSuccess = "SUCCESS"
	CKYCStatusFailed  = "FAILED"
	CKYCStatusBlocked = "BLOCKED" // no CKYC provider configured for this environment
)

// CKYCRequest tracks one CKYC/CERSAI registry interaction. The full
// registry response is never stored verbatim — only a reference number,
// status, and a short failure reason, mirroring BureauRequest's approach to
// not hoarding sensitive third-party payloads in the primary DB.
type CKYCRequest struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CustomerID uuid.UUID `gorm:"type:uuid;index;not null" json:"customerId"`

	RequestType string `gorm:"not null" json:"requestType"`
	Status      string `gorm:"not null" json:"status"`

	CKYCReference string `json:"ckycReference,omitempty"` // the registry's own KYC number, once known
	ProviderRef   string `json:"providerRef,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	RetryCount    int    `json:"retryCount"`

	RequestedAt time.Time  `json:"requestedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}
