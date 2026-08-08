package profile

type PANRequest struct {
	PANNumber string `json:"pan_number" binding:"required"`
	NameOnPAN string `json:"name_on_pan" binding:"required"`
}