package bureau

import (
	"hash/fnv"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

// sandboxProvider is the exact same deterministic-per-customer generator
// this codebase always used for its "CIBIL score" — kept, not deleted,
// because a sandbox/development environment still needs SOME stable
// score to exercise underwriting logic against without a real bureau
// account. The only change is that it is now clearly labeled SANDBOX
// (models.BureauRequest.Bureau/Provider), gated to never activate in
// production (see service.go Initialize), and reached only through
// PullScore's consent-and-audit path instead of being called inline from
// loan handlers.
type sandboxProvider struct{}

func (s *sandboxProvider) Name() string { return models.BureauSandbox }

func (s *sandboxProvider) PullScore(customerID uuid.UUID) (Result, error) {
	h := fnv.New32a()
	h.Write([]byte(customerID.String()))
	sum := h.Sum32()
	score := 550 + int(sum%301) // 550..850 inclusive

	return Result{
		Score:          score,
		RiskIndicators: "sandbox-generated, not a real bureau assessment",
		ProviderRef:    "SANDBOX-" + customerID.String(),
		RawResponse:    "sandbox-deterministic-score:" + customerID.String(),
	}, nil
}
