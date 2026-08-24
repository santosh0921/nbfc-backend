package collections

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/rbac"
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

// AdminListCasesHandler — item I. A COLLECTION_OFFICER sees only cases
// assigned to them (?mine=1 or the officer's own admin_id implied when no
// explicit assignedTo filter is given and the caller lacks
// CollectionManage) — enforced here rather than relying solely on the
// admin panel to filter, so a curious officer can't just remove a query
// param to browse every other officer's book. A COLLECTION_MANAGER (who
// holds CollectionManage) sees everything.
func AdminListCasesHandler(c *gin.Context) {
	q := database.DB.Model(&models.CollectionCase{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	canManage := rbac.Has(c.GetString("admin_role"), rbac.CollectionManage)
	if !canManage {
		adminID, _ := currentAdmin(c)
		q = q.Where("assigned_officer_admin_id = ?", adminID)
	} else if assignedTo := c.Query("assignedTo"); assignedTo != "" {
		q = q.Where("assigned_officer_admin_id = ?", assignedTo)
	}
	var cases []models.CollectionCase
	q.Order("next_action_date asc nulls last, id desc").Limit(200).Find(&cases)
	c.JSON(http.StatusOK, cases)
}

func AdminGetCaseHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var caseRow models.CollectionCase
	if err := database.DB.First(&caseRow, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Case not found"})
		return
	}
	var actions []models.CollectionAction
	database.DB.Where("case_id = ?", id).Order("created_at desc").Find(&actions)
	var notes []models.CollectionNote
	database.DB.Where("case_id = ?", id).Order("created_at desc").Find(&notes)
	var promises []models.CollectionPromise
	database.DB.Where("case_id = ?", id).Order("created_at desc").Find(&promises)
	c.JSON(http.StatusOK, gin.H{"case": caseRow, "actions": actions, "notes": notes, "promises": promises})
}

// AdminAssignCaseHandler — requires COLLECTION_MANAGE (item L).
func AdminAssignCaseHandler(c *gin.Context) {
	caseID, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		AssignToAdminID uint   `json:"assignToAdminId" binding:"required"`
		AssignToName    string `json:"assignToName" binding:"required"`
		Reason          string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, adminName := currentAdmin(c)
	result, err := AssignCase(uint(caseID), body.AssignToAdminID, body.AssignToName, adminID, adminName, body.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// AdminEscalateCaseHandler — item I/J. Available to any authorized
// collection actor (officer or manager) — only assignment is manager-only.
func AdminEscalateCaseHandler(c *gin.Context) {
	caseID, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)
	adminID, adminName := currentAdmin(c)
	result, err := EscalateCase(uint(caseID), adminID, adminName, body.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// AdminRecordActionHandler — item J. Never touches any financial field
// (item I: "Do not allow collection staff to modify accounting balances
// directly") — the request body has no amount/balance field at all.
func AdminRecordActionHandler(c *gin.Context) {
	caseID, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		ActionType       string     `json:"actionType" binding:"required"`
		Result           string     `json:"result"`
		Notes            string     `json:"notes"`
		CommunicationRef string     `json:"communicationRef"`
		NextAction       *time.Time `json:"nextAction"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, adminName := currentAdmin(c)
	action, err := RecordAction(uint(caseID), adminID, adminName, body.ActionType, body.Result, body.Notes, body.CommunicationRef, body.NextAction)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, action)
}

func AdminAddNoteHandler(c *gin.Context) {
	caseID, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, adminName := currentAdmin(c)
	note := models.CollectionNote{CaseID: uint(caseID), ActorAdminID: adminID, ActorName: adminName, Note: body.Note}
	database.DB.Create(&note)
	c.JSON(http.StatusCreated, note)
}

// AdminCreatePromiseHandler — item K. Recording a promise never itself
// settles anything.
func AdminCreatePromiseHandler(c *gin.Context) {
	caseID, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		LoanID          uint      `json:"loanId" binding:"required"`
		PromisedAmountPaise int64 `json:"promisedAmountPaise" binding:"required"`
		PromisedDate    time.Time `json:"promisedDate" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	adminID, adminName := currentAdmin(c)
	promise, err := CreatePromise(uint(caseID), body.LoanID, body.PromisedAmountPaise, body.PromisedDate, adminID, adminName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, promise)
}
