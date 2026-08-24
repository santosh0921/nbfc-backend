package finance

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentAdmin(c *gin.Context) (uint, string) {
	var id uint
	if v, ok := c.Get("admin_id"); ok {
		if cast, ok := v.(uint); ok {
			id = cast
		}
	}
	return id, c.GetString("admin_username")
}

// --- Vendors ---

func AdminListVendorsHandler(c *gin.Context) {
	var vendors []models.Vendor
	database.DB.Order("name").Find(&vendors)
	c.JSON(http.StatusOK, vendors)
}

func AdminCreateVendorHandler(c *gin.Context) {
	var body struct {
		Name              string `json:"name" binding:"required"`
		Category          string `json:"category"`
		PAN               string `json:"pan"`
		GSTIN             string `json:"gstin"`
		BankAccountMasked string `json:"bankAccountMasked"`
		IFSC              string `json:"ifsc"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	v, err := CreateVendor(body.Name, body.Category, body.PAN, body.GSTIN, body.BankAccountMasked, body.IFSC, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, v)
}

func AdminSetVendorStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := SetVendorStatus(uint(id), body.Status, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// --- Expenses ---

func AdminListExpensesHandler(c *gin.Context) {
	q := database.DB.Model(&models.Expense{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var expenses []models.Expense
	q.Order("id desc").Limit(500).Find(&expenses)
	c.JSON(http.StatusOK, expenses)
}

// AdminCreateExpenseHandler binds into a narrow request struct rather
// than models.Expense directly — binding the full model would let a
// client set Status/ApprovedBy/JournalReference/PaymentReference etc.
// (OWASP mass assignment, flagged as a general risk in the Phase 7
// audit) — every one of those fields is set only by CreateExpense/
// ApproveExpense/MarkPaid, never accepted from the request body.
func AdminCreateExpenseHandler(c *gin.Context) {
	var body struct {
		ExpenseDate   time.Time `json:"expenseDate"`
		VendorID      *uint     `json:"vendorId"`
		Category      string    `json:"category" binding:"required"`
		Description   string    `json:"description"`
		InvoiceNumber string    `json:"invoiceNumber"`
		GrossAmountPaise int64  `json:"grossAmountPaise" binding:"required"`
		TaxAmountPaise   int64  `json:"taxAmountPaise"`
		TDSAmountPaise   int64  `json:"tdsAmountPaise"`
		AccountCode      string `json:"accountCode" binding:"required"`
		SupportingDocumentRef string `json:"supportingDocumentRef"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	expense := &models.Expense{
		ExpenseDate: body.ExpenseDate, VendorID: body.VendorID, Category: body.Category,
		Description: body.Description, InvoiceNumber: body.InvoiceNumber,
		GrossAmountPaise: body.GrossAmountPaise, TaxAmountPaise: body.TaxAmountPaise, TDSAmountPaise: body.TDSAmountPaise,
		AccountCode: body.AccountCode, SupportingDocumentRef: body.SupportingDocumentRef,
	}
	adminID, actor := currentAdmin(c)
	e, err := CreateExpense(expense, adminID, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, e)
}

func AdminApproveExpenseHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, actor := currentAdmin(c)
	e, err := ApproveExpense(uint(id), adminID, actor)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, e)
}

func AdminRejectExpenseHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)
	adminID, actor := currentAdmin(c)
	e, err := RejectExpense(uint(id), adminID, actor, body.Reason)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, e)
}

func AdminMarkExpensePaidHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		PaymentReference string `json:"paymentReference"`
	}
	c.ShouldBindJSON(&body)
	_, actor := currentAdmin(c)
	e, err := MarkPaid(uint(id), body.PaymentReference, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, e)
}

func AdminReverseExpenseHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, actor := currentAdmin(c)
	e, err := ReverseExpense(uint(id), adminID, actor, body.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, e)
}

// --- Bank balances ---

func AdminBankBalancesHandler(c *gin.Context) {
	asOf := time.Now()
	if dateStr := c.Query("asOf"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOf = t
		}
	}
	c.JSON(http.StatusOK, AllBankAccountSummaries(asOf))
}

func AdminBankBalanceHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	asOf := time.Now()
	if dateStr := c.Query("asOf"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOf = t
		}
	}
	summary, err := BankAccountBalance(uint(id), asOf)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Bank account not found"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// --- Financial dashboard ---

func AdminFinancialDashboardHandler(c *gin.Context) {
	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	c.JSON(http.StatusOK, GenerateFinancialDashboard(from, to))
}
