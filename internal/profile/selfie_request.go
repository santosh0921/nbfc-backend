package profile

type SelfieRequest struct {
	Image string `json:"image" binding:"required"`
}