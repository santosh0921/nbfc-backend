package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/admin"
	"github.com/santosh0921/nbfc-backend/internal/agency"
	"github.com/santosh0921/nbfc-backend/internal/alerts"
	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/auth"
	"github.com/santosh0921/nbfc-backend/internal/documents"
	"github.com/santosh0921/nbfc-backend/internal/employee"
	"github.com/santosh0921/nbfc-backend/internal/loans"
	"github.com/santosh0921/nbfc-backend/internal/middleware"
	"github.com/santosh0921/nbfc-backend/internal/notifications"
	"github.com/santosh0921/nbfc-backend/internal/otp"
	"github.com/santosh0921/nbfc-backend/internal/profile"
	"github.com/santosh0921/nbfc-backend/internal/recovery"
	"github.com/santosh0921/nbfc-backend/internal/signaling"
	"github.com/santosh0921/nbfc-backend/internal/support"
)

func RegisterRoutes(router *gin.Engine) {

	// authRateLimit: simple in-memory per-IP token bucket (1 req/sec,
	// burst 5) applied to the public auth endpoints most prone to abuse
	// (OTP spam / credential stuffing). See internal/middleware/rate_limit.go.
	authRateLimit := middleware.RateLimit(1, 5)

	// Public Routes
	//
	// verify-otp and create-mpin used to be the only two public auth
	// endpoints WITHOUT authRateLimit — verify-otp guards OTP brute-force
	// (otp.VerifyOTP's 5-attempt cap was otherwise trivially bypassed by
	// just calling send-otp again for a fresh counter, since nothing
	// throttled how often that loop could repeat), and create-mpin is
	// effectively a password-reset endpoint for an existing account, which
	// should never be unthrottled regardless of what credential it checks.
	router.POST("/auth/send-otp", authRateLimit, otp.SendOTPHandler)
	router.POST("/auth/verify-otp", authRateLimit, otp.VerifyOTPHandler)
	router.POST("/auth/create-mpin", authRateLimit, auth.CreateMPINHandler)
	router.POST("/auth/login", authRateLimit, auth.LoginHandler)


	// Protected Routes
	authorized := router.Group("/auth")
	authorized.Use(middleware.AuthMiddleware())

	authorized.GET("/profile", auth.GetProfile)
	authorized.POST("/profile", profile.CreateProfileHandler)
	authorized.POST("/address", profile.CreateAddressHandler)
	authorized.POST("/pan", profile.CreatePANHandler)
	authorized.POST("/aadhaar", profile.CreateAadhaarHandler)
	authorized.POST("/selfie", profile.CreateSelfieHandler)
	authorized.GET("/kyc/status", profile.GetKYCStatusHandler)
	authorized.POST("/employment", profile.CreateEmploymentHandler)

	authorized.PUT("/biometric", auth.ToggleBiometric)

	authorized.POST("/documents", documents.UploadCustomerDocumentHandler)
	authorized.GET("/documents", documents.ListCustomerDocumentsHandler)

	// Admin Routes
	router.POST("/admin/login", authRateLimit, admin.LoginHandler)

	adminGroup := router.Group("/admin")
	adminGroup.Use(middleware.AdminAuthMiddleware())

	adminGroup.GET("/dashboard", admin.DashboardHandler)
	adminGroup.GET("/users", admin.ListUsersHandler)
	adminGroup.GET("/users/:id", admin.GetUserDetailHandler)
	adminGroup.POST("/users/:id/kyc/approve", admin.ApproveKYCHandler)
	adminGroup.POST("/users/:id/kyc/reject", admin.RejectKYCHandler)

	adminGroup.GET("/loans", loans.AdminListLoansHandler)
	adminGroup.GET("/loans/:id", loans.AdminGetLoanHandler)
	adminGroup.POST("/loans/:id/decision", loans.AdminDecisionHandler)
	adminGroup.POST("/loans/:id/disburse", loans.AdminDisburseHandler)
	adminGroup.GET("/loans/:id/documents", documents.AdminListLoanDocumentsHandler)
	adminGroup.GET("/loans/:id/emi-schedule", loans.AdminEmiScheduleHandler)
	adminGroup.GET("/customers/:id/documents", documents.AdminListCustomerDocumentsHandler)

	adminGroup.GET("/recovery-reports", recovery.AdminListReportsHandler)
	adminGroup.POST("/recovery-reports/:id/action", recovery.AdminActionHandler)

	adminGroup.GET("/agencies", agency.ListHandler)
	adminGroup.POST("/agencies", agency.CreateHandler)
	adminGroup.DELETE("/agencies/:id", agency.DeleteHandler)

	adminGroup.GET("/employees", employee.AdminListHandler)
	adminGroup.POST("/employees", employee.AdminCreateHandler)
	adminGroup.POST("/employees/:id/approve", employee.AdminApproveHandler)
	adminGroup.DELETE("/employees/:id", employee.AdminDeleteHandler)

	adminGroup.GET("/audit-log", audit.AdminListHandler)
	adminGroup.GET("/sla-alerts", alerts.AdminSlaAlertsHandler)
	adminGroup.POST("/loans/:id/restructure", loans.AdminRestructureLoanHandler)

	adminGroup.GET("/support/threads", support.AdminListThreadsHandler)
	adminGroup.GET("/support/threads/:id/messages", support.AdminThreadMessagesHandler)
	adminGroup.POST("/support/threads/:id/messages", support.AdminReplyHandler)
	adminGroup.POST("/support/threads/:id/close", support.AdminCloseThreadHandler)

	// Customer loan routes (reuse existing customer JWT middleware)
	authorized.POST("/loans/apply", loans.SubmitLoanHandler)
	authorized.GET("/loans/mine", loans.ListMyLoansHandler)
	authorized.GET("/loans/:id", loans.GetMyLoanHandler)
	authorized.GET("/loans/:id/emi-schedule", loans.CustomerEmiScheduleHandler)
	authorized.POST("/loans/:id/emi/:installmentId/pay", loans.CustomerPayEmiHandler)
	authorized.GET("/dashboard/emi-summary", loans.CustomerEmiSummaryHandler)
	authorized.POST("/loans/:id/topup", loans.TopUpLoanHandler)
	authorized.POST("/loans/:id/esign", loans.CustomerEsignLetterHandler)
	authorized.GET("/transactions", loans.CustomerTransactionsHandler)

	authorized.POST("/support/messages", support.PostMessageHandler)
	authorized.GET("/support/messages", support.ListMessagesHandler)

	// Employee routes (new employee JWT middleware)
	router.POST("/employee/login", authRateLimit, employee.LoginHandler)
	router.POST("/employee/register", authRateLimit, employee.RegisterHandler)

	employeeGroup := router.Group("/employee")
	employeeGroup.Use(middleware.EmployeeAuthMiddleware())

	employeeGroup.GET("/tasks/today", loans.EmployeeTasksTodayHandler)
	employeeGroup.POST("/loans/:id/verify", loans.EmployeeVerifyLoanHandler)
	employeeGroup.POST("/loans/:id/documents", documents.UploadEmployeeLoanDocumentHandler)
	employeeGroup.GET("/recovery/cases/today", recovery.CasesTodayHandler)
	employeeGroup.POST("/recovery/cases/:id/report", recovery.SubmitReportHandler)

	// Notifications: accepts either a customer JWT or an employee JWT,
	// inferring the recipient (role + id) from whichever token validates.
	router.GET("/notifications", middleware.CustomerOrEmployeeAuthMiddleware(), notifications.ListHandler)
	// WebRTC signaling relay for employee<->customer video calls — see
	// internal/signaling/handler.go's doc comment for what this endpoint
	// does and doesn't do.
	router.GET("/ws/call/:loanId", middleware.CustomerOrEmployeeAuthMiddleware(), signaling.CallSignalHandler)
	router.POST("/notifications/:id/read", middleware.CustomerOrEmployeeAuthMiddleware(), notifications.MarkReadHandler)
}