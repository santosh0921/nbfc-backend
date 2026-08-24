package reconciliation

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// AdminListBankAccountsHandler / AdminCreateBankAccountHandler — the
// small setup step a reconciliation import is scoped against.
func AdminListBankAccountsHandler(c *gin.Context) {
	var accounts []models.BankAccount
	database.DB.Order("created_at asc").Find(&accounts)
	c.JSON(http.StatusOK, accounts)
}

func AdminCreateBankAccountHandler(c *gin.Context) {
	var body struct {
		AccountName         string `json:"accountName" binding:"required"`
		AccountNumberMasked string `json:"accountNumberMasked" binding:"required"`
		IFSC                string `json:"ifsc"`
		BankName            string `json:"bankName"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	account := models.BankAccount{
		AccountName: body.AccountName, AccountNumberMasked: body.AccountNumberMasked,
		IFSC: body.IFSC, BankName: body.BankName, Active: true,
	}
	if err := database.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create bank account"})
		return
	}
	c.JSON(http.StatusCreated, account)
}

// AdminListBankTransactionsHandler handles GET /admin/reconciliation/transactions
// — the "Unmatched Transactions" / general reconciliation table, with the
// filters the admin panel needs.
func AdminListBankTransactionsHandler(c *gin.Context) {
	q := database.DB.Model(&models.BankTransaction{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if bankAccountID := c.Query("bankAccountId"); bankAccountID != "" {
		q = q.Where("bank_account_id = ?", bankAccountID)
	}
	if reference := c.Query("reference"); reference != "" {
		q = q.Where("reference ILIKE ?", "%"+reference+"%")
	}
	if from := c.Query("from"); from != "" {
		q = q.Where("txn_date >= ?", from)
	}
	if to := c.Query("to"); to != "" {
		q = q.Where("txn_date <= ?", to)
	}

	limit := 200
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 1000 {
		limit = v
	}

	var total int64
	q.Count(&total)
	var txns []models.BankTransaction
	q.Order("txn_date desc").Limit(limit).Find(&txns)
	c.JSON(http.StatusOK, gin.H{"transactions": txns, "total": total})
}

// AdminListReconciliationExceptionsHandler handles
// GET /admin/reconciliation/exceptions
func AdminListReconciliationExceptionsHandler(c *gin.Context) {
	q := database.DB.Model(&models.ReconciliationException{})
	if c.Query("unresolvedOnly") == "true" {
		q = q.Where("resolved_at IS NULL")
	}
	var exceptions []models.ReconciliationException
	q.Order("created_at desc").Find(&exceptions)
	c.JSON(http.StatusOK, exceptions)
}

type manualMatchRequest struct {
	MatchedEntityType string `json:"matchedEntityType" binding:"required"` // "disbursement" | "payment"
	MatchedEntityID   uint   `json:"matchedEntityId" binding:"required"`
	Reason            string `json:"reason" binding:"required"`
}

// AdminManualMatchHandler handles
// POST /admin/reconciliation/transactions/:id/match — requires
// RECONCILIATION_MANAGE. Every manual match requires a reason (enforced
// by binding:"required"), records the acting admin + timestamp, and — for
// a disbursement match — actually confirms the disbursement/loan via the
// same internal/loans.ConfirmDisbursement path an auto-match uses.
func AdminManualMatchHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body manualMatchRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "matchedEntityType, matchedEntityId, and reason are all required"})
		return
	}
	if body.MatchedEntityType != "disbursement" && body.MatchedEntityType != "payment" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "matchedEntityType must be \"disbursement\" or \"payment\""})
		return
	}

	var txn models.BankTransaction
	if err := database.DB.First(&txn, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Bank transaction not found"})
		return
	}
	if txn.Status == models.BankTxnStatusReconciled {
		c.JSON(http.StatusConflict, gin.H{"message": "This transaction is already reconciled"})
		return
	}

	adminIDVal, _ := c.Get("admin_id")
	adminID, _ := adminIDVal.(uint)
	adminUsername := c.GetString("admin_username")

	applyMatch(&txn, body.MatchedEntityType, body.MatchedEntityID, "manual", adminUsername, &adminID, body.Reason)

	// A manual match resolves any open exception for this transaction too.
	now := time.Now()
	database.DB.Model(&models.ReconciliationException{}).
		Where("bank_transaction_id = ? AND resolved_at IS NULL", txn.ID).
		Updates(map[string]interface{}{
			"resolved_at": now, "resolved_by": adminUsername, "resolved_by_admin_id": adminID,
			"resolution_note": "Resolved via manual match: " + body.Reason,
		})

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: c.GetString("admin_role"),
		Action: "reconciliation.manual_match", TargetType: "bank_transaction", TargetID: strconv.Itoa(int(txn.ID)),
		Result: audit.ResultSuccess, Reason: body.Reason,
		Details: body.MatchedEntityType + " #" + strconv.Itoa(int(body.MatchedEntityID)),
	})

	database.DB.First(&txn, txn.ID)
	c.JSON(http.StatusOK, txn)
}

type resolveExceptionRequest struct {
	Note string `json:"note" binding:"required"`
}

// AdminResolveExceptionHandler handles
// POST /admin/reconciliation/exceptions/:id/resolve — requires
// RECONCILIATION_MANAGE. Marks an exception resolved WITHOUT necessarily
// creating a match (e.g. "this is a bank charge, not a loan-related
// transaction, no match needed") — for actually matching an exception's
// transaction to an entity, use AdminManualMatchHandler instead, which
// resolves the exception as a side effect.
func AdminResolveExceptionHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body resolveExceptionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A resolution note is required"})
		return
	}

	var exception models.ReconciliationException
	if err := database.DB.First(&exception, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if exception.ResolvedAt != nil {
		c.JSON(http.StatusConflict, gin.H{"message": "Already resolved"})
		return
	}

	adminIDVal, _ := c.Get("admin_id")
	adminID, _ := adminIDVal.(uint)
	adminUsername := c.GetString("admin_username")
	now := time.Now()

	exception.ResolvedAt = &now
	exception.ResolvedBy = adminUsername
	exception.ResolvedByAdminID = &adminID
	exception.ResolutionNote = body.Note
	database.DB.Save(&exception)

	database.DB.Model(&models.BankTransaction{}).Where("id = ?", exception.BankTransactionID).
		Update("status", models.BankTxnStatusManuallyResolved)

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: adminUsername, ActorRole: c.GetString("admin_role"),
		Action: "reconciliation.exception_resolved", TargetType: "reconciliation_exception", TargetID: strconv.Itoa(int(exception.ID)),
		Result: audit.ResultSuccess, Reason: body.Note,
	})

	c.JSON(http.StatusOK, exception)
}
