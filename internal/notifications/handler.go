package notifications

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/push"
)

// Create is the one place a Notification row should ever be created —
// it persists the in-app inbox row AND fires a real device push (best-
// effort; a missing/invalid FCM token or push provider never blocks
// this) to every device the recipient has registered. Callers that used
// to build &models.Notification{...} directly (internal/loans/service.go,
// internal/loans/noc.go, internal/recovery/handler.go) call this instead
// so every notification reaches a locked/backgrounded device, not just
// the in-app inbox someone has to think to open.
func Create(role, userID, title, body string) {
	n := models.Notification{Role: role, UserID: userID, Title: title, Body: body}
	database.DB.Create(&n)
	push.SendToUser(role, userID, title, body)
}

// AdminBroadcastHandler handles POST /admin/notifications/broadcast — an
// admin-composed announcement (loan offers, app-wide banners, ...) sent
// to every customer and/or employee who has ever registered a device
// token, i.e. everyone with the app actually installed and logged in at
// least once. Doubles as the simplest possible real-world push-delivery
// test: send one, then check whether it actually shows up on a device
// with the app closed — that's a genuine FCM round-trip, not a
// simulation.
//
// Recipients are read from DeviceToken (not the User/Employee tables)
// specifically so this never wastes a push attempt on someone who has no
// registered device to receive it on. Deduplicated by (role, userId) so
// a customer with the app on two phones gets one in-app notification
// row, not two — internal/push.SendToUser already fans out to every
// token that customer has.
func AdminBroadcastHandler(c *gin.Context) {
	var body struct {
		Title    string `json:"title" binding:"required"`
		Body     string `json:"body" binding:"required"`
		Audience string `json:"audience"` // "customer" | "employee" | "" (both)
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	query := database.DB.Model(&models.DeviceToken{})
	if body.Audience == "customer" || body.Audience == "employee" {
		query = query.Where("role = ?", body.Audience)
	}
	var tokens []models.DeviceToken
	query.Find(&tokens)

	seen := map[string]bool{}
	recipients := 0
	for _, t := range tokens {
		key := t.Role + "|" + t.UserID
		if seen[key] {
			continue
		}
		seen[key] = true
		Create(t.Role, t.UserID, body.Title, body.Body)
		recipients++
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Broadcast sent",
		"recipients": recipients,
	})
}

// resolveRecipient inspects context values set by whichever auth middleware
// ran (customer or employee JWT) to infer the notification recipient.
func resolveRecipient(c *gin.Context) (role string, userID string, ok bool) {
	if empID := c.GetUint("employee_id"); empID != 0 {
		return "employee", strconv.Itoa(int(empID)), true
	}
	if mobile := c.GetString("mobile"); mobile != "" {
		var user models.User
		if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err == nil {
			return "customer", user.ID.String(), true
		}
	}
	return "", "", false
}

// ListHandler handles GET /notifications
func ListHandler(c *gin.Context) {
	role, userID, ok := resolveRecipient(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	// Non-nil start — see the identical fix note in
	// loans.ListMyLoansHandler: a nil slice marshals to JSON `null`, which
	// the client's list decoder throws on instead of showing "No
	// notifications yet".
	notes := []models.Notification{}
	database.DB.Where("role = ? AND user_id = ?", role, userID).Order("created_at desc").Find(&notes)
	c.JSON(http.StatusOK, notes)
}

// MarkReadHandler handles POST /notifications/:id/read
func MarkReadHandler(c *gin.Context) {
	role, userID, ok := resolveRecipient(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var n models.Notification
	if err := database.DB.Where("id = ? AND role = ? AND user_id = ?", id, role, userID).First(&n).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	n.Read = true
	database.DB.Save(&n)
	c.JSON(http.StatusOK, n)
}

// ===================== Admin-scoped notifications (Phase 7) =====================
//
// The KYC portal's notification bell was calling the customer/employee
// routes above, which only ever resolve a recipient via
// CustomerOrEmployeeAuthMiddleware — an admin JWT never parses there, so
// every admin request 401'd. Rather than teach the shared
// customer/employee middleware (or resolveRecipient) about a third actor
// type — which would touch behavior for the Customer and Employee apps —
// these are separate, additive routes gated by AdminAuthMiddleware, reusing
// the same Notification model/table and the same Create() helper with
// role="admin". No new table, no change to existing customer/employee
// notification behavior.

// AdminListHandler handles GET /admin/notifications
func AdminListHandler(c *gin.Context) {
	adminID, ok := c.Get("admin_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	userID := strconv.Itoa(int(adminID.(uint)))
	notes := []models.Notification{}
	database.DB.Where("role = ? AND user_id = ?", "admin", userID).Order("created_at desc").Find(&notes)
	c.JSON(http.StatusOK, notes)
}

// AdminMarkReadHandler handles POST /admin/notifications/:id/read
func AdminMarkReadHandler(c *gin.Context) {
	adminIDVal, ok := c.Get("admin_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	userID := strconv.Itoa(int(adminIDVal.(uint)))
	id, _ := strconv.Atoi(c.Param("id"))
	var n models.Notification
	if err := database.DB.Where("id = ? AND role = ? AND user_id = ?", id, "admin", userID).First(&n).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
		return
	}
	n.Read = true
	database.DB.Save(&n)
	c.JSON(http.StatusOK, n)
}

// AdminMarkAllReadHandler handles POST /admin/notifications/mark-all-read
func AdminMarkAllReadHandler(c *gin.Context) {
	adminIDVal, ok := c.Get("admin_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	userID := strconv.Itoa(int(adminIDVal.(uint)))
	database.DB.Model(&models.Notification{}).
		Where("role = ? AND user_id = ? AND read = ?", "admin", userID, false).
		Update("read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}

// MarkAllReadHandler handles POST /notifications/mark-all-read — the
// bell icon's "mark all read" action, so a customer/employee with a
// backlog of unread notifications doesn't have to tap through each one.
func MarkAllReadHandler(c *gin.Context) {
	role, userID, ok := resolveRecipient(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	database.DB.Model(&models.Notification{}).
		Where("role = ? AND user_id = ? AND read = ?", role, userID, false).
		Update("read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}

type registerTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"`
}

// RegisterTokenHandler handles POST /auth/device-token and
// POST /employee/device-token — called once on login and again whenever
// Firebase rotates the token (onTokenRefresh), so this must be safe to
// call repeatedly, not just once per install.
//
// A raw FCM token is deleted and re-inserted under the current caller
// rather than upserted in place: the same token can legitimately move
// between accounts (a shared/test device logging out of one account and
// into another), and deleting first guarantees a stale row from a
// previous owner can never linger and receive pushes meant for someone
// else.
func RegisterTokenHandler(c *gin.Context) {
	role, userID, ok := resolveRecipient(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	var req registerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "token is required"})
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "android"
	}

	database.DB.Where("token = ?", token).Delete(&models.DeviceToken{})
	dt := models.DeviceToken{Role: role, UserID: userID, Token: token, Platform: platform}
	if err := database.DB.Create(&dt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to register device"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "registered"})
}
