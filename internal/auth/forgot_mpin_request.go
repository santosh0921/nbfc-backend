package auth

type ForgotMPINRequest struct {
	Mobile      string `json:"mobile" binding:"required,len=10,numeric"`
	NewMPIN     string `json:"new_mpin" binding:"required,len=6,numeric"`
	ConfirmMPIN string `json:"confirm_mpin" binding:"required,len=6,numeric"`
}