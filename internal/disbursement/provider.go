// Package disbursement provides the provider abstraction and record-
// keeping for actually moving sanctioned loan money out to a borrower —
// distinct from (and downstream of) the Phase 1 maker-checker approval
// trail on LoanApplication itself. A checker approving a disbursement
// (internal/loans/maker_checker_handler.go) creates a Disbursement here
// and asks a Provider to initiate the transfer; the loan does not become
// DISBURSED until that transfer is independently confirmed complete
// (internal/loans.ConfirmDisbursement, called either by a real payout
// provider's webhook or, in this deployment where no live payout-API
// credentials exist, by internal/reconciliation matching the transfer's
// UTR against an imported bank statement line).
package disbursement

import "time"

// TransferResult is what CreateTransfer returns once a provider has
// accepted (not necessarily completed) a transfer request.
type TransferResult struct {
	ProviderReference string
	Status            string // one of the models.DisbursementStatus* constants
}

// StatusResult is what CheckStatus returns when polling a provider for an
// already-initiated transfer's current state.
type StatusResult struct {
	Status        string
	UTR           string
	CompletedAt   *time.Time
	FailureReason string
}

// Provider is the seam a real bank/payout API (RazorpayX, Cashfree
// Payouts, a bank's own corporate-banking API, etc.) plugs into.
// ManualBankTransferProvider (below) is the only implementation active in
// this deployment, since no live payout-API credentials are configured —
// swapping in a real one later means implementing this interface, not
// restructuring the maker-checker workflow that calls it.
type Provider interface {
	Name() string
	CreateTransfer(amountPaise int64, bankAccountReference, reference string) (TransferResult, error)
	CheckStatus(providerReference string) (StatusResult, error)
	// HandleWebhook is a placeholder seam for a provider that pushes
	// transfer-status updates — unused by ManualBankTransferProvider,
	// which has no webhook (confirmation instead comes from bank
	// reconciliation, see internal/reconciliation).
	HandleWebhook(rawBody []byte, signatureHeader string) (StatusResult, string, error)
}

var activeProvider Provider = &ManualBankTransferProvider{}

// Active returns the configured disbursement provider. Always non-nil —
// ManualBankTransferProvider is a legitimate, real-world disbursement
// method (an NBFC operator initiates the NEFT/RTGS through their own
// bank's portal, outside any API this codebase controls), not a mock; it
// is the correct default for a deployment with no payout-API credentials
// configured, rather than refusing to allow disbursement at all.
func Active() Provider {
	return activeProvider
}
