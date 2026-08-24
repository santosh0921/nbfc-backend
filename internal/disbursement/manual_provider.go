package disbursement

import (
	"errors"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

// ManualBankTransferProvider models the real, common-for-smaller-NBFCs
// disbursement method: an authorized operator initiates the actual
// NEFT/RTGS/IMPS transfer through the NBFC's own bank's portal (outside
// this codebase's control — there is no API to call), and the transfer's
// completion is confirmed afterward by matching its UTR against an
// imported bank statement (internal/reconciliation). CreateTransfer here
// only records that intent; it does not and cannot move money itself.
type ManualBankTransferProvider struct{}

func (m *ManualBankTransferProvider) Name() string { return "manual_bank_transfer" }

func (m *ManualBankTransferProvider) CreateTransfer(amountPaise int64, bankAccountReference, reference string) (TransferResult, error) {
	// No external call — the "provider reference" is the merchant-side
	// reference itself, since there is no external system issuing one.
	// Status starts at INITIATED: the operator still has to actually go
	// perform the bank transfer outside this system, and reconciliation
	// is what later confirms it happened.
	return TransferResult{
		ProviderReference: reference,
		Status:            models.DisbursementStatusInitiated,
	}, nil
}

func (m *ManualBankTransferProvider) CheckStatus(providerReference string) (StatusResult, error) {
	// Nothing to poll — this provider has no external system with its own
	// status. Confirmation only ever comes from reconciliation.
	return StatusResult{}, errors.New("manual_bank_transfer has no status API — confirm via bank reconciliation instead")
}

func (m *ManualBankTransferProvider) HandleWebhook(rawBody []byte, signatureHeader string) (StatusResult, string, error) {
	return StatusResult{}, "", errors.New("manual_bank_transfer does not receive webhooks")
}
