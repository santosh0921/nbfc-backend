package profile

type CreateProfileRequest struct {
	FirstName     string `json:"first_name" binding:"required"`
	LastName      string `json:"last_name" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	DateOfBirth   string `json:"date_of_birth" binding:"required"`
	Gender        string `json:"gender" binding:"required"`
	MaritalStatus string `json:"marital_status" binding:"required"`
	Occupation    string `json:"occupation" binding:"required"`
}