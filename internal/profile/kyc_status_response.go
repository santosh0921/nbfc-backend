package profile

type KYCSection struct {
	Completed bool   `json:"completed"`
	Status    string `json:"status,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type KYCStatusResponse struct {
	Profile KYCSection `json:"profile"`
	Address KYCSection `json:"address"`

	PAN      KYCSection `json:"pan"`
	Aadhaar  KYCSection `json:"aadhaar"`
	Selfie   KYCSection `json:"selfie"`

	OverallKYC           string `json:"overall_kyc"`
	CompletionPercentage int    `json:"completion_percentage"`

	NextStep string `json:"next_step,omitempty"`

}