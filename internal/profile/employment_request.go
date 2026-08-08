package profile

type EmploymentRequest struct {
	EmploymentType string  `json:"employment_type" binding:"required"`
	CompanyName    string  `json:"company_name" binding:"required"`
	Occupation     string  `json:"occupation" binding:"required"`
	MonthlyIncome  float64 `json:"monthly_income" binding:"required"`
	WorkEmail      string  `json:"work_email"`
	ExperienceYears int    `json:"experience_years"`
	SalaryDay      int     `json:"salary_day"`
}