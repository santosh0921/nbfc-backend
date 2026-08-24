// Package bureau implements the CreditBureauService abstraction (Phase 3,
// item F): a provider-configurable seam for CIBIL/Experian/CRIF/Equifax,
// with consent enforcement (item G) and a versioned underwriting policy
// that consumes bureau data without letting the bureau provider directly
// determine loan approval (item H).
//
// Only one concrete provider exists today: sandboxProvider, the same
// deterministic per-customer score internal/loans used to compute inline
// (see git history of internal/loans/reference.go) — moved here and
// explicitly gated to never run when config.IsProduction() is true. No
// real bureau (CIBIL/Experian/CRIF/Equifax) credentials are configured in
// this environment; PullScore returns a clear "bureau not configured"
// error in production rather than silently falling back to the sandbox
// score, per the Phase 3 instruction not to let a fake score reach a real
// credit decision path.
package bureau

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/config"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Result is what a Provider returns for a completed pull.
type Result struct {
	Score          int
	RiskIndicators string
	ProviderRef    string
	RawResponse    string // hashed before storage, never persisted verbatim
}

// Provider is the seam a real bureau API plugs into.
type Provider interface {
	Name() string
	PullScore(customerID uuid.UUID) (Result, error)
}

var ErrConsentRequired = errors.New("bureau: no valid consent on file for this customer/bureau — collect consent before pulling a report")
var ErrProviderNotConfigured = errors.New("bureau: no credit bureau provider is configured for this environment")

// activeProvider defaults to the sandbox provider outside production, and
// to nil (nothing configured) in production — see IsProduction check in
// PullScore, which refuses to use the sandbox provider even if somehow
// still set, as defense in depth beyond just what's assigned here.
var activeProvider Provider

func Initialize() {
	if !config.IsProduction() {
		activeProvider = &sandboxProvider{}
	}
	// Production: no real bureau credentials configured in this
	// environment. activeProvider stays nil — PullScore reports
	// ErrProviderNotConfigured rather than ever silently using the
	// sandbox generator.
}

// RecordConsent stores a BureauConsent row — must be called (and succeed)
// before PullScore will do anything for that customer/bureau pair.
func RecordConsent(customerID uuid.UUID, bureauName, purpose, consentVersion, applicationRef, ip, device string) (*models.BureauConsent, error) {
	consent := models.BureauConsent{
		CustomerID: customerID, Bureau: bureauName, Purpose: purpose,
		ConsentVersion: consentVersion, ApplicationRef: applicationRef,
		IPAddress: ip, DeviceInfo: device, ConsentedAt: time.Now(),
	}
	if err := database.DB.Create(&consent).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "customer", ActorRef: customerID.String(),
		Action: "bureau.consent", TargetType: "bureau_consent", TargetID: consent.ID.String(),
		Result: audit.ResultSuccess, Details: bureauName + " / " + purpose,
	})
	return &consent, nil
}

// latestConsent finds the most recent consent for this customer+bureau.
// No expiry window is enforced today (a documented gap — see the Phase 3
// report; a real deployment should define and enforce a consent validity
// period, a business/compliance decision this code cannot make on its
// own).
func latestConsent(customerID uuid.UUID, bureauName string) (*models.BureauConsent, error) {
	var consent models.BureauConsent
	err := database.DB.Where("customer_id = ? AND bureau = ?", customerID, bureauName).
		Order("consented_at desc").First(&consent).Error
	if err != nil {
		return nil, ErrConsentRequired
	}
	return &consent, nil
}

// PullScore is THE single entry point for obtaining a credit score in
// this codebase — requires an existing consent, records a BureauRequest
// either way (success or failure), and never lets a provider failure or
// missing configuration silently produce a usable score.
func PullScore(customerID uuid.UUID, bureauName string) (*models.BureauRequest, error) {
	consent, err := latestConsent(customerID, bureauName)
	if err != nil {
		return nil, err
	}

	req := models.BureauRequest{
		CustomerID: customerID, Bureau: bureauName, RequestType: "score",
		ConsentID: consent.ID, Status: models.BureauRequestStatusPending, RequestedAt: time.Now(),
	}
	database.DB.Create(&req)

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: bureauName,
		Action: "bureau.request", TargetType: "bureau_request", TargetID: req.ID.String(),
		Result: audit.ResultSuccess, Details: "customer " + customerID.String(),
	})

	provider := activeProvider
	if config.IsProduction() {
		// Defense in depth: even if activeProvider were somehow non-nil in
		// production (it shouldn't be — Initialize never sets it there),
		// refuse the sandbox provider by name explicitly.
		if provider == nil || provider.Name() == models.BureauSandbox {
			provider = nil
		}
	}
	if provider == nil {
		req.Status = models.BureauRequestStatusBlocked
		req.FailureReason = ErrProviderNotConfigured.Error()
		database.DB.Save(&req)
		recordResponseAudit(req, false)
		return &req, ErrProviderNotConfigured
	}

	result, err := provider.PullScore(customerID)
	now := time.Now()
	if err != nil {
		req.Status = models.BureauRequestStatusFailed
		req.FailureReason = err.Error()
		req.CompletedAt = &now
		database.DB.Save(&req)
		recordResponseAudit(req, false)
		return &req, err
	}

	sum := sha256.Sum256([]byte(result.RawResponse))
	req.Status = models.BureauRequestStatusCompleted
	req.Score = result.Score
	req.RiskIndicators = result.RiskIndicators
	req.ProviderReference = result.ProviderRef
	req.ResponseHash = hex.EncodeToString(sum[:])
	req.CompletedAt = &now
	database.DB.Save(&req)
	recordResponseAudit(req, true)

	return &req, nil
}

func recordResponseAudit(req models.BureauRequest, success bool) {
	result := audit.ResultFailure
	if success {
		result = audit.ResultSuccess
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "system", ActorRef: req.Bureau,
		Action: "bureau.response", TargetType: "bureau_request", TargetID: req.ID.String(),
		Result: result, Reason: req.FailureReason,
	})
}
