package models

import "time"

type RecoveryCase struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	LoanRef            string    `json:"loanRef"`
	CustomerName       string    `json:"customerName"`
	CustomerPhone      string    `json:"customerPhone"`
	OverdueAmount      float64   `json:"overdueAmount"`
	City               string    `json:"city"`
	AssignedEmployeeID uint      `json:"assignedEmployeeId"`
	Status             string    `json:"status"` // open, reported, resolved, escalated
	CreatedAt          time.Time `json:"createdAt"`
}

type RecoveryReport struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	CaseID         uint      `json:"caseId"`
	StatusUpdate   string    `json:"statusUpdate"`
	Remarks        string    `json:"remarks"`
	RiskAssessment string    `json:"riskAssessment"`
	Recommendation string    `json:"recommendation"`
	NextAction     string    `json:"nextAction"`
	CreatedAt      time.Time `json:"createdAt"`
}

type CaseNote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CaseID    uint      `json:"caseId"`
	Author    string    `json:"author"`
	Action    string    `json:"action"`
	Remarks   string    `json:"remarks"`
	CreatedAt time.Time `json:"createdAt"`
}
