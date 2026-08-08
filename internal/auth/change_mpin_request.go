package auth

type ChangeMPINRequest struct {
	CurrentMPIN string `json:"current_mpin" binding:"required,len=6,numeric"`
	NewMPIN     string `json:"new_mpin" binding:"required,len=6,numeric"`
	ConfirmMPIN string `json:"confirm_mpin" binding:"required,len=6,numeric"`
}