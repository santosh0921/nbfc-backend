package surepass

type PANResponse struct {
	Data struct {
		ClientID       string `json:"client_id"`
		PANNumber      string `json:"pan_number"`
		FullName       string `json:"full_name"`
		MaskedAadhaar  string `json:"masked_aadhaar"`
		Email          string `json:"email"`
		PhoneNumber    string `json:"phone_number"`
		Gender         string `json:"gender"`
		DOB            string `json:"dob"`
		AadhaarLinked  bool   `json:"aadhaar_linked"`
		Category       string `json:"category"`
	} `json:"data"`

	StatusCode int    `json:"status_code"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	MessageCode string `json:"message_code"`
}