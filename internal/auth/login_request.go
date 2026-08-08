package auth

type LoginRequest struct {
	Mobile string `json:"mobile" binding:"required,len=10,numeric"`
	MPIN   string `json:"mpin" binding:"required,len=6,numeric"`
}