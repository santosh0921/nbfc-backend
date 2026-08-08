package profile

type AadhaarRequest struct {
	AadhaarNumber string `json:"aadhaar_number" binding:"required"`
	NameOnAadhaar string `json:"name_on_aadhaar" binding:"required"`

	Consent bool `json:"consent" binding:"required"`
}