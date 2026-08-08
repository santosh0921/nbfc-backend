package constants

const (
	VerificationPending  = "PENDING"
	VerificationVerified = "VERIFIED"
	VerificationFailed   = "FAILED"
	VerificationNameMismatch = "NAME_MISMATCH"


	KYCPending  = "PENDING"
	KYCApproved = "APPROVED"
	KYCRejected = "REJECTED"

	// Loan application statuses now live on models.LoanApplication and use
	// the models.LoanStatus* constants (submitted/auto_verified/assigned/
	// verified/sanctioned/rejected/disbursed). These KYC-review-era Loan*
	// constants are kept only if something else still references them.
)