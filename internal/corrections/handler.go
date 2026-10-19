package corrections

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/customerinfo"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

type correctionRow struct {
	models.CorrectionCase
	CustomerName  string `json:"customerName,omitempty"`
	CustomerMobile string `json:"customerMobile,omitempty"`
	// CurrentDocCompleteness is only populated on the detail endpoint
	// (list stays cheap — no per-row extra queries) and only for KYC
	// domain — lets the Correction Portal warn when nothing has changed
	// since DocCompletenessAtRaise, without blocking a resolve over it.
	CurrentDocCompleteness *int `json:"currentDocCompleteness,omitempty"`
}

func currentAdmin(c *gin.Context) (uint, string, string) {
	id, _ := c.Get("admin_id")
	idVal, _ := id.(uint)
	return idVal, c.GetString("admin_username"), c.GetString("admin_role")
}

// AdminListHandler handles GET /admin/corrections — perm CORRECTION_VIEW.
// Optional ?domain=KYC|CREDIT, ?status=OPEN|IN_PROGRESS|RESOLVED,
// ?category= filters; defaults to OPEN+IN_PROGRESS (the actual work
// queue) when no status is given, mirroring the KYC Queue's own default.
func AdminListHandler(c *gin.Context) {
	q := database.DB.Model(&models.CorrectionCase{})
	if domain := c.Query("domain"); domain != "" {
		q = q.Where("domain = ?", strings.ToUpper(domain))
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", strings.ToUpper(status))
	} else {
		q = q.Where("status IN ?", []string{models.CorrectionStatusOpen, models.CorrectionStatusInProgress})
	}
	if category := c.Query("category"); category != "" {
		q = q.Where("loan_category = ?", category)
	}

	var cases []models.CorrectionCase
	q.Order("created_at asc").Find(&cases)

	ids := make([]uuid.UUID, len(cases))
	for i, cc := range cases {
		ids[i] = cc.CustomerID
	}
	names := customerinfo.BatchNames(ids)

	var users []models.User
	if len(ids) > 0 {
		database.DB.Where("id IN ?", ids).Find(&users)
	}
	mobiles := make(map[uuid.UUID]string, len(users))
	for _, u := range users {
		mobiles[u.ID] = u.Mobile
	}

	out := make([]correctionRow, len(cases))
	for i, cc := range cases {
		out[i] = correctionRow{CorrectionCase: cc, CustomerName: names[cc.CustomerID], CustomerMobile: mobiles[cc.CustomerID]}
	}
	c.JSON(http.StatusOK, out)
}

// AdminDetailHandler handles GET /admin/corrections/:id — perm
// CORRECTION_VIEW. Customer documents are deliberately NOT duplicated
// here — the portal fetches them from the existing
// GET /admin/customers/:id/documents endpoint directly, same source of
// truth every other portal uses, with its own audit-logged access record.
func AdminDetailHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid correction id"})
		return
	}
	var cc models.CorrectionCase
	if err := database.DB.First(&cc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	names := customerinfo.BatchNames([]uuid.UUID{cc.CustomerID})
	var user models.User
	mobile := ""
	if database.DB.First(&user, "id = ?", cc.CustomerID).Error == nil {
		mobile = user.Mobile
	}
	row := correctionRow{CorrectionCase: cc, CustomerName: names[cc.CustomerID], CustomerMobile: mobile}
	if cc.Domain == models.CorrectionDomainKYC {
		current := documentCompleteness(cc.CustomerID)
		row.CurrentDocCompleteness = &current
	}
	c.JSON(http.StatusOK, row)
}

// AdminClaimHandler handles POST /admin/corrections/:id/claim — perm
// CORRECTION_MANAGE. Assigns the case to the calling admin and moves it
// to IN_PROGRESS, same "claim" pattern as the field-employee loan queue.
func AdminClaimHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid correction id"})
		return
	}
	var cc models.CorrectionCase
	if err := database.DB.First(&cc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if cc.Status == models.CorrectionStatusResolved {
		c.JSON(http.StatusConflict, gin.H{"message": "This correction is already resolved"})
		return
	}

	adminID, username, _ := currentAdmin(c)
	before := audit.Snapshot(cc)
	cc.Status = models.CorrectionStatusInProgress
	cc.AssignedToAdminID = &adminID
	cc.AssignedToUsername = username
	if err := database.DB.Save(&cc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to claim"})
		return
	}
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "correction.claim", TargetType: "correction_case", TargetID: strconv.Itoa(int(cc.ID)),
		Result: audit.ResultSuccess, Details: before, After: cc,
	})
	c.JSON(http.StatusOK, cc)
}

type resolveRequest struct {
	Notes string `json:"notes" binding:"required"`
}

// AdminResolveHandler handles POST /admin/corrections/:id/resolve — perm
// CORRECTION_MANAGE. Re-opens the originating KYC/Credit review (see
// Resolve) so the case naturally reappears in that queue for a second
// look — the correction employee's job ends at "the customer is fixable
// now", not at re-deciding KYC/credit themselves.
func AdminResolveHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid correction id"})
		return
	}
	var cc models.CorrectionCase
	if err := database.DB.First(&cc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	if cc.Status == models.CorrectionStatusResolved {
		c.JSON(http.StatusConflict, gin.H{"message": "Already resolved"})
		return
	}

	var body resolveRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Resolution notes are required"})
		return
	}

	adminID, username, _ := currentAdmin(c)
	before := audit.Snapshot(cc)
	if err := Resolve(&cc, adminID, username, body.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to resolve"})
		return
	}
	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: adminID, ActorRef: username,
		Action: "correction.resolve", TargetType: "correction_case", TargetID: strconv.Itoa(int(cc.ID)),
		Result: audit.ResultSuccess, Reason: body.Notes, Details: before, After: cc,
	})
	c.JSON(http.StatusOK, cc)
}
