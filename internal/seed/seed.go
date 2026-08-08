package seed

import (
	"log"

	"golang.org/x/crypto/bcrypt"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func hashPw(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	return string(h)
}

// Run seeds agencies, employees, and sample recovery cases. It is idempotent
// (guarded by count checks) so it is safe to call on every startup. Admin
// accounts are NOT seeded here — admin auth is a single hardcoded .env user.
func Run() {
	db := database.DB

	var agCount int64
	db.Model(&models.Agency{}).Count(&agCount)
	if agCount == 0 {
		agencies := []models.Agency{
			{Name: "Thane Branch", City: "Thane", Address: "LBS Road, Thane", Active: true},
			{Name: "Mumbai Branch", City: "Mumbai", Address: "Andheri East, Mumbai", Active: true},
			{Name: "Pune Branch", City: "Pune", Address: "FC Road, Pune", Active: true},
			{Name: "Bengaluru Branch", City: "Bengaluru", Address: "MG Road, Bengaluru", Active: true},
			{Name: "Chennai Branch", City: "Chennai", Address: "Anna Salai, Chennai", Active: true},
			{Name: "Delhi Branch", City: "Delhi", Address: "Connaught Place, Delhi", Active: true},
			{Name: "Hyderabad Branch", City: "Hyderabad", Address: "Banjara Hills, Hyderabad", Active: true},
			{Name: "Ahmedabad Branch", City: "Ahmedabad", Address: "CG Road, Ahmedabad", Active: true},
		}
		db.Create(&agencies)
		log.Println("seeded agencies")
	}

	var empCount int64
	db.Model(&models.Employee{}).Count(&empCount)
	if empCount == 0 {
		var agencies []models.Agency
		db.Find(&agencies)
		agByCity := map[string]uint{}
		for _, a := range agencies {
			agByCity[a.City] = a.ID
		}
		pw := hashPw("onefin123")
		emps := []models.Employee{
			{Code: "VER-THN-01", Name: "Rajesh Sharma", PasswordHash: pw, Role: "verification", BranchCity: "Thane", AgencyID: agByCity["Thane"], Active: true},
			{Code: "VER-MUM-01", Name: "Priya Deshmukh", PasswordHash: pw, Role: "verification", BranchCity: "Mumbai", AgencyID: agByCity["Mumbai"], Active: true},
			{Code: "VER-PUN-01", Name: "Amit Kulkarni", PasswordHash: pw, Role: "verification", BranchCity: "Pune", AgencyID: agByCity["Pune"], Active: true},
			{Code: "VER-BLR-01", Name: "Suresh Reddy", PasswordHash: pw, Role: "verification", BranchCity: "Bengaluru", AgencyID: agByCity["Bengaluru"], Active: true},
			{Code: "VER-CHN-01", Name: "Karthik Iyer", PasswordHash: pw, Role: "verification", BranchCity: "Chennai", AgencyID: agByCity["Chennai"], Active: true},
			{Code: "REC-DEL-01", Name: "Vikram Singh", PasswordHash: pw, Role: "recovery", BranchCity: "Delhi", AgencyID: agByCity["Delhi"], Active: true},
			{Code: "REC-HYD-01", Name: "Anita Rao", PasswordHash: pw, Role: "recovery", BranchCity: "Hyderabad", AgencyID: agByCity["Hyderabad"], Active: true},
			{Code: "REC-AHM-01", Name: "Manish Patel", PasswordHash: pw, Role: "recovery", BranchCity: "Ahmedabad", AgencyID: agByCity["Ahmedabad"], Active: true},
		}
		db.Create(&emps)
		log.Println("seeded employees (password: onefin123)")
	}

	var rcCount int64
	db.Model(&models.RecoveryCase{}).Count(&rcCount)
	if rcCount == 0 {
		var recEmps []models.Employee
		db.Where("role = ?", "recovery").Find(&recEmps)
		if len(recEmps) > 0 {
			cases := []models.RecoveryCase{
				{LoanRef: "LN-1001", CustomerName: "Sunil Verma", CustomerPhone: "9811122233", OverdueAmount: 45000, City: "Delhi", AssignedEmployeeID: recEmps[0].ID, Status: "open"},
				{LoanRef: "LN-1002", CustomerName: "Farida Khan", CustomerPhone: "9822233344", OverdueAmount: 78000, City: "Hyderabad", AssignedEmployeeID: recEmps[1%len(recEmps)].ID, Status: "open"},
				{LoanRef: "LN-1003", CustomerName: "Ramesh Gupta", CustomerPhone: "9833344455", OverdueAmount: 15000, City: "Ahmedabad", AssignedEmployeeID: recEmps[2%len(recEmps)].ID, Status: "open"},
			}
			db.Create(&cases)
			log.Println("seeded recovery cases")
		}
	}
}
