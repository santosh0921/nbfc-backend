package profile

type AddressRequest struct {
	CurrentAddress  string `json:"current_address" binding:"required"`
	CurrentState    string `json:"current_state" binding:"required"`
	CurrentDistrict string `json:"current_district" binding:"required"`
	CurrentCity     string `json:"current_city" binding:"required"`
	CurrentPincode  string `json:"current_pincode" binding:"required"`

	SameAsCurrent bool `json:"same_as_current"`

	PermanentAddress  string `json:"permanent_address"`
	PermanentState    string `json:"permanent_state"`
	PermanentDistrict string `json:"permanent_district"`
	PermanentCity     string `json:"permanent_city"`
	PermanentPincode  string `json:"permanent_pincode"`
}