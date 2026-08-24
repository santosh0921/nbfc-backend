package accounting

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func adminActor(c *gin.Context) string {
	if v := c.GetString("admin_username"); v != "" {
		return v
	}
	return "system"
}

// --- Chart of Accounts (item A) ---

func AdminListAccountsHandler(c *gin.Context) {
	var accounts []models.LedgerAccount
	database.DB.Order("account_code").Find(&accounts)
	c.JSON(http.StatusOK, accounts)
}

func AdminCreateAccountHandler(c *gin.Context) {
	var body models.LedgerAccount
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.AccountCode == "" || body.AccountName == "" || body.AccountType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "accountCode, accountName, and accountType are required"})
		return
	}
	switch body.AccountType {
	case models.AccountTypeAsset, models.AccountTypeLiability, models.AccountTypeEquity, models.AccountTypeIncome, models.AccountTypeExpense:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"message": "accountType must be one of ASSET/LIABILITY/EQUITY/INCOME/EXPENSE"})
		return
	}
	if body.Status == "" {
		body.Status = models.AccountStatusActive
	}
	if err := database.DB.Create(&body).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"message": "Failed to create account (duplicate code?)"})
		return
	}
	c.JSON(http.StatusCreated, body)
}

func AdminUpdateAccountHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var acct models.LedgerAccount
	if err := database.DB.First(&acct, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Account not found"})
		return
	}
	var body struct {
		AccountName string `json:"accountName"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.AccountName != "" {
		acct.AccountName = body.AccountName
	}
	if body.Description != "" {
		acct.Description = body.Description
	}
	if body.Status == models.AccountStatusActive || body.Status == models.AccountStatusInactive {
		acct.Status = body.Status
	}
	database.DB.Save(&acct)
	c.JSON(http.StatusOK, acct)
}

// --- Journal entries ---

func AdminListJournalEntriesHandler(c *gin.Context) {
	q := database.DB.Model(&models.JournalEntry{}).Preload("Lines")
	if refType := c.Query("referenceType"); refType != "" {
		q = q.Where("reference_type = ?", refType)
	}
	if refID := c.Query("referenceId"); refID != "" {
		q = q.Where("reference_id = ?", refID)
	}
	var entries []models.JournalEntry
	q.Order("id desc").Limit(200).Find(&entries)
	c.JSON(http.StatusOK, entries)
}

func AdminGetJournalEntryHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var entry models.JournalEntry
	if err := database.DB.Preload("Lines").First(&entry, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Journal entry not found"})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// AdminPostManualJournalHandler handles POST /admin/accounting/journal —
// requires ACCOUNTING_POST. Used for adjustments/corrections that don't
// come from an automated business-event posting function — still funneled
// through the same Service.PostJournalEntry (balance-validated,
// idempotency-keyed, period-checked) as every other posting in this
// codebase.
func AdminPostManualJournalHandler(c *gin.Context) {
	var body struct {
		Description string      `json:"description" binding:"required"`
		Lines       []LineInput `json:"lines" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	actor := adminActor(c)
	entry, err := Service.PostJournalEntry(PostingInput{
		TransactionType: "MANUAL_ADJUSTMENT",
		ReferenceType:   "manual",
		ReferenceID:     strconv.FormatInt(time.Now().UnixNano(), 10),
		Description:     body.Description,
		CreatedBy:       actor,
		ApprovedBy:      actor,
		Lines:           body.Lines,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// AdminReverseJournalHandler handles POST /admin/accounting/journal/:id/reverse
// — requires ACCOUNTING_POST. The only way to correct a posted entry.
func AdminReverseJournalHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	reversal, err := ReverseJournalEntry(uint(id), body.Reason, adminActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, reversal)
}

// --- Accounting periods (item M) ---

func AdminListPeriodsHandler(c *gin.Context) {
	var periods []models.AccountingPeriod
	database.DB.Order("code desc").Find(&periods)
	c.JSON(http.StatusOK, periods)
}

func AdminClosePeriodHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	period, err := ClosePeriod(uint(id), adminActor(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, period)
}

func AdminReopenPeriodHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	period, err := ReopenPeriod(uint(id), adminActor(c), body.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, period)
}

// --- Reports (items N/O/P/Q) — all server-side, date/period filtered ---

func parseDateRange(c *gin.Context) (from, to time.Time, ok bool) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	var err error
	if fromStr == "" || toStr == "" {
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = now
		return from, to, true
	}
	from, err = time.Parse("2006-01-02", fromStr)
	if err != nil {
		return
	}
	to, err = time.Parse("2006-01-02", toStr)
	if err != nil {
		return
	}
	to = to.Add(24*time.Hour - time.Second)
	return from, to, true
}

func AdminTrialBalanceHandler(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid from/to date (expected YYYY-MM-DD)"})
		return
	}
	c.JSON(http.StatusOK, TrialBalance(from, to))
}

func AdminProfitAndLossHandler(c *gin.Context) {
	from, to, ok := parseDateRange(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid from/to date (expected YYYY-MM-DD)"})
		return
	}
	c.JSON(http.StatusOK, ProfitAndLoss(from, to))
}

func AdminBalanceSheetHandler(c *gin.Context) {
	_, to, ok := parseDateRange(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid from/to date (expected YYYY-MM-DD)"})
		return
	}
	c.JSON(http.StatusOK, BalanceSheet(to))
}

func AdminGeneralLedgerHandler(c *gin.Context) {
	accountCode := c.Query("accountCode")
	if accountCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "accountCode is required"})
		return
	}
	from, to, ok := parseDateRange(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid from/to date (expected YYYY-MM-DD)"})
		return
	}
	lines, err := GeneralLedger(accountCode, from, to)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lines)
}

// --- GST / invoices (items H/I) ---

func AdminListGSTTransactionsHandler(c *gin.Context) {
	var txns []models.GSTTransaction
	database.DB.Order("id desc").Limit(200).Find(&txns)
	c.JSON(http.StatusOK, txns)
}

func AdminListInvoicesHandler(c *gin.Context) {
	var invoices []models.TaxInvoice
	database.DB.Order("id desc").Limit(200).Find(&invoices)
	c.JSON(http.StatusOK, invoices)
}

// --- TDS (item J) — read/status-management only; nothing auto-applies TDS ---

func AdminListTDSHandler(c *gin.Context) {
	var txns []models.TDSTransaction
	database.DB.Order("id desc").Limit(200).Find(&txns)
	c.JSON(http.StatusOK, txns)
}

// --- Borrowings/funding (item K) ---

func AdminListFundingSourcesHandler(c *gin.Context) {
	var sources []models.FundingSource
	database.DB.Order("id desc").Find(&sources)
	c.JSON(http.StatusOK, sources)
}

func AdminCreateFundingSourceHandler(c *gin.Context) {
	var body models.FundingSource
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.LenderName == "" || body.PrincipalPaise <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "lenderName and a positive principalPaise are required"})
		return
	}
	body.OutstandingPaise = body.PrincipalPaise
	body.Status = models.FundingStatusActive
	database.DB.Create(&body)

	entry, err := Service.PostJournalEntry(PostingInput{
		TransactionType: "BORROWING_DRAWDOWN",
		ReferenceType:   "funding_source",
		ReferenceID:     strconv.Itoa(int(body.ID)),
		Description:     "Drawdown from " + body.LenderName,
		CreatedBy:       adminActor(c),
		Lines: []LineInput{
			{AccountCode: AcctBankCash, DebitPaise: body.PrincipalPaise, Description: "Funds received"},
			{AccountCode: AcctBorrowingsPayable, CreditPaise: body.PrincipalPaise, Description: "Borrowing principal payable"},
		},
	})
	if err == nil {
		body.AccountingReference = entry.JournalNumber
		database.DB.Save(&body)
	}
	c.JSON(http.StatusCreated, body)
}
