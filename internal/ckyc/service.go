// Package ckyc implements the CKYC/CERSAI provider abstraction (Phase 3,
// item J). No CKYC registry credentials/entity certification are available
// in this environment — CERSAI access requires a registered-entity
// certificate and onboarding process this codebase cannot self-provision.
// activeProvider therefore stays nil in every environment today; every
// call is recorded as a real CKYCRequest row with status BLOCKED rather
// than a fake SUCCESS/CKYC number. Do not fake CKYC numbers or responses —
// see the Phase 3 report's "awaiting credentials" section.
package ckyc

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Result is what a real Provider implementation would return.
type Result struct {
	CKYCReference string
	ProviderRef   string
}

// Provider is the seam a certified CERSAI/CKYC integration plugs into.
type Provider interface {
	Name() string
	Search(customerID uuid.UUID) (Result, error)
	Retrieve(ckycReference string) (Result, error)
	CreateOrUpdate(customerID uuid.UUID, requestType string) (Result, error)
}

var ErrProviderNotConfigured = errors.New("ckyc: no CKYC/CERSAI provider is configured — requires a certified registered-entity integration this environment does not have")

// activeProvider is never set today — see the package doc comment. Kept as
// a var (not a hardcoded nil check) so a future real provider can be wired
// in exactly the way internal/bureau's Initialize does, without any caller
// changes.
var activeProvider Provider

func Initialize() {
	// Intentionally left unset: no CKYC provider available in this
	// environment. See package doc comment.
}

// Request is the single entry point for any CKYC interaction — always
// writes a CKYCRequest row (BLOCKED when no provider is configured) and an
// audit event, never silently skips either.
func Request(customerID uuid.UUID, requestType string) (*models.CKYCRequest, error) {
	req := models.CKYCRequest{
		CustomerID: customerID, RequestType: requestType,
		Status: models.CKYCStatusPending, RequestedAt: time.Now(),
	}
	database.DB.Create(&req)

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: "ckyc",
		Action: "ckyc.request", TargetType: "ckyc_request", TargetID: req.ID.String(),
		Result: audit.ResultSuccess, Details: requestType + " for customer " + customerID.String(),
	})

	if activeProvider == nil {
		req.Status = models.CKYCStatusBlocked
		req.FailureReason = ErrProviderNotConfigured.Error()
		database.DB.Save(&req)
		recordResponseAudit(req, false)
		return &req, ErrProviderNotConfigured
	}

	var result Result
	var err error
	switch requestType {
	case models.CKYCRequestTypeSearch:
		result, err = activeProvider.Search(customerID)
	case models.CKYCRequestTypeRetrieve:
		result, err = activeProvider.Retrieve(req.CKYCReference)
	default:
		result, err = activeProvider.CreateOrUpdate(customerID, requestType)
	}

	now := time.Now()
	if err != nil {
		req.Status = models.CKYCStatusFailed
		req.FailureReason = err.Error()
		req.RetryCount++
		req.CompletedAt = &now
		database.DB.Save(&req)
		recordResponseAudit(req, false)
		return &req, err
	}

	req.Status = models.CKYCStatusSuccess
	req.CKYCReference = result.CKYCReference
	req.ProviderRef = result.ProviderRef
	req.CompletedAt = &now
	database.DB.Save(&req)
	recordResponseAudit(req, true)
	return &req, nil
}

func recordResponseAudit(req models.CKYCRequest, success bool) {
	result := audit.ResultFailure
	if success {
		result = audit.ResultSuccess
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: "ckyc",
		Action: "ckyc.response", TargetType: "ckyc_request", TargetID: req.ID.String(),
		Result: result, Reason: req.FailureReason,
	})
}
