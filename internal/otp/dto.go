package otp

type SendOTPRequest struct {
	Mobile string `json:"mobile" binding:"required,len=10"`
}

type VerifyOTPRequest struct {
	Mobile string `json:"mobile" binding:"required,len=10"`
	OTP    string `json:"otp" binding:"required,len=6"`
}