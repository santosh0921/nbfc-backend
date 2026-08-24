package tax

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

// --- Rules (maker-checker) ---

func AdminListRulesHandler(c *gin.Context) {
	var rules []models.TDSRule
	database.DB.Order("id desc").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

func AdminDraftRuleHandler(c *gin.Context) {
	var rule models.TDSRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	r, err := DraftRule(&rule, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, r)
}

func AdminApproveRuleHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	adminID, actor := currentAdmin(c)
	r, err := ApproveRule(uint(id), adminID, actor)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrSameActor {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}

func AdminRetireRuleHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, actor := currentAdmin(c)
	if err := RetireRule(uint(id), actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "retired"})
}

// --- Calculation ---

func AdminCalculateTDSHandler(c *gin.Context) {
	var body struct {
		ReferenceType    string    `json:"referenceType" binding:"required"`
		ReferenceID      string    `json:"referenceId" binding:"required"`
		DeducteeID       *uint     `json:"deducteeId"`
		DeducteeName     string    `json:"deducteeName" binding:"required"`
		GrossAmountPaise int64     `json:"grossAmountPaise" binding:"required"`
		NatureOfPayment  string    `json:"natureOfPayment" binding:"required"`
		PANStatus        string    `json:"panStatus"`
		TransactionDate  time.Time `json:"transactionDate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.TransactionDate.IsZero() {
		body.TransactionDate = time.Now()
	}
	if body.PANStatus == "" {
		body.PANStatus = models.PANStatusNotProvided
	}
	_, actor := currentAdmin(c)
	txn, err := CalculateTDS(CalculateInput{
		ReferenceType: body.ReferenceType, ReferenceID: body.ReferenceID,
		DeducteeID: body.DeducteeID, DeducteeName: body.DeducteeName,
		GrossAmountPaise: body.GrossAmountPaise, NatureOfPayment: body.NatureOfPayment,
		PANStatus: body.PANStatus, TransactionDate: body.TransactionDate, Actor: actor,
	})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, txn)
}

func AdminListTransactionsHandler(c *gin.Context) {
	q := database.DB.Model(&models.TDSTransaction{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.TDSTransaction
	q.Order("id desc").Limit(500).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminOverrideTDSHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		NewTDSAmountPaise int64  `json:"newTdsAmountPaise" binding:"required"`
		Reason            string `json:"reason" binding:"required"`
		ApprovedBy        string `json:"approvedBy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, actor := currentAdmin(c)
	txn, err := OverrideTDS(uint(id), body.NewTDSAmountPaise, body.Reason, adminID, actor, body.ApprovedBy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, txn)
}

func AdminPostDeductionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		ExpenseAccountCode string `json:"expenseAccountCode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	txn, err := PostTDSDeduction(uint(id), body.ExpenseAccountCode, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, txn)
}

func AdminReverseDeductionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	txn, err := ReverseTDSDeduction(uint(id), body.Reason, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, txn)
}

// --- Challans ---

func AdminListChallansHandler(c *gin.Context) {
	var rows []models.TDSChallan
	database.DB.Order("id desc").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateChallanHandler(c *gin.Context) {
	var challan models.TDSChallan
	if err := c.ShouldBindJSON(&challan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	ch, err := CreateChallan(&challan, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ch)
}

func AdminLinkChallanHandler(c *gin.Context) {
	var body struct {
		TDSTransactionID uint `json:"tdsTransactionId" binding:"required"`
		ChallanID        uint `json:"challanId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := LinkTransactionToChallan(body.TDSTransactionID, body.ChallanID, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "linked"})
}

func AdminMarkChallanPaidHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		ActualDepositDate time.Time `json:"actualDepositDate" binding:"required"`
		BankReference     string    `json:"bankReference" binding:"required"`
		ChallanNumber     string    `json:"challanNumber"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	ch, err := MarkChallanPaid(uint(id), body.ActualDepositDate, body.BankReference, body.ChallanNumber, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ch)
}

// --- Deductees ---

func AdminListDeducteesHandler(c *gin.Context) {
	var rows []models.TDSDeductee
	database.DB.Order("name").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateDeducteeHandler(c *gin.Context) {
	var body struct {
		Name           string `json:"name" binding:"required"`
		VendorID       *uint  `json:"vendorId"`
		PAN            string `json:"pan"`
		ResidentStatus string `json:"residentStatus"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	d, err := CreateDeductee(body.Name, body.VendorID, body.PAN, body.ResidentStatus, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, d)
}

func AdminSetPANStatusHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status        string `json:"status" binding:"required"`
		Source        string `json:"source" binding:"required"`
		FailureReason string `json:"failureReason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	if err := SetPANValidationStatus(uint(id), body.Status, body.Source, body.FailureReason, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// --- Exceptions ---

func AdminListExceptionsHandler(c *gin.Context) {
	q := database.DB.Model(&models.TDSException{})
	if c.Query("all") != "1" {
		q = q.Where("status = ?", models.TDSExceptionStatusOpen)
	}
	var rows []models.TDSException
	q.Order("id desc").Limit(300).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminRunReconciliationHandler(c *gin.Context) {
	found := RunTDSReconciliation()
	c.JSON(http.StatusOK, gin.H{"findingsChecked": found})
}

func AdminResolveExceptionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Resolution string `json:"resolution"`
		Evidence   string `json:"evidence"`
	}
	c.ShouldBindJSON(&body)
	_, actor := currentAdmin(c)
	if err := ResolveException(uint(id), actor, body.Resolution, body.Evidence); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "resolved"})
}

// --- Compliance calendar ---

func AdminListCalendarHandler(c *gin.Context) {
	var rows []models.TaxComplianceCalendarItem
	database.DB.Order("due_date").Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func AdminCreateCalendarItemHandler(c *gin.Context) {
	var item models.TaxComplianceCalendarItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_, actor := currentAdmin(c)
	created, err := CreateCalendarItem(&item, actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func AdminCompleteCalendarItemHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		ReviewerName string `json:"reviewerName"`
		Evidence     string `json:"evidence"`
	}
	c.ShouldBindJSON(&body)
	_, actor := currentAdmin(c)
	if err := MarkCalendarItemComplete(uint(id), body.ReviewerName, body.Evidence, actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "completed"})
}

// --- Dashboard ---

func AdminTaxDashboardHandler(c *gin.Context) {
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
	c.JSON(http.StatusOK, GenerateTaxDashboard(from, to))
}
