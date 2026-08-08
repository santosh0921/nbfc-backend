package verification

type VerificationResult struct {
	IsVerified bool

	Status string

	Provider string

	Reference string

	FailureReason string

	Remarks string

	NameMatched bool
}

