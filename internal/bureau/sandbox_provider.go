package bureau

import (
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

// sandboxProvider previously generated a deterministic-per-customer score
// spread across 550..850 — with models.CibilRejectThreshold at 650, that
// meant roughly a third of test customers got hash-generated scores that
// hard-rejected their applications for no reason a tester could control,
// which defeated the point of a sandbox: everyone should be able to apply
// and exercise the full underwriting/approval flow. It now returns a
// fixed favorable score (prime tier, > models.CibilPrimeThreshold) for
// every customer instead. Still clearly labeled SANDBOX
// (models.BureauRequest.Bureau/Provider), still gated to never activate in
// production (see service.go Initialize), and still reached only through
// PullScore's consent-and-audit path rather than being called inline from
// loan handlers.
type sandboxProvider struct{}

func (s *sandboxProvider) Name() string { return models.BureauSandbox }

// defaultSandboxScore is comfortably above models.CibilPrimeThreshold so
// every sandbox applicant qualifies for the minimum rate and is never
// blocked by the CIBIL gate in evaluateEligibility.
const defaultSandboxScore = 780

func (s *sandboxProvider) PullScore(customerID uuid.UUID) (Result, error) {
	return Result{
		Score:          defaultSandboxScore,
		RiskIndicators: "sandbox-generated, not a real bureau assessment",
		ProviderRef:    "SANDBOX-" + customerID.String(),
		RawResponse:    "sandbox-fixed-default-score:" + customerID.String(),
	}, nil
}
