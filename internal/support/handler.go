package support

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func currentUserID(c *gin.Context) (string, bool) {
	mobile := c.GetString("mobile")
	if mobile == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return "", false
	}
	var user models.User
	if err := database.DB.Where("mobile = ?", mobile).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return "", false
	}
	return user.ID.String(), true
}

// findOrCreateOpenThread returns the customer's existing open thread, or
// creates a new one if none exists.
func findOrCreateOpenThread(customerID string) (models.SupportThread, error) {
	var thread models.SupportThread
	err := database.DB.Where("customer_id = ? AND status = ?", customerID, models.SupportThreadOpen).
		Order("updated_at desc").First(&thread).Error
	if err == nil {
		return thread, nil
	}
	cid, parseErr := uuid.Parse(customerID)
	if parseErr != nil {
		return thread, parseErr
	}
	thread = models.SupportThread{CustomerID: cid, Status: models.SupportThreadOpen}
	createErr := database.DB.Create(&thread).Error
	return thread, createErr
}

// PostMessageHandler handles POST /auth/support/messages
func PostMessageHandler(c *gin.Context) {
	customerID, ok := currentUserID(c)
	if !ok {
		return
	}
	var body struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	thread, err := findOrCreateOpenThread(customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to open support thread"})
		return
	}

	msg := models.SupportMessage{
		ThreadID:   thread.ID,
		SenderType: models.SupportSenderCustomer,
		SenderRef:  customerID,
		Body:       body.Body,
	}
	if err := database.DB.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to send message"})
		return
	}
	database.DB.Model(&thread).Update("updated_at", msg.CreatedAt)

	c.JSON(http.StatusCreated, msg)
}

// ListMessagesHandler handles GET /auth/support/messages
func ListMessagesHandler(c *gin.Context) {
	customerID, ok := currentUserID(c)
	if !ok {
		return
	}
	var thread models.SupportThread
	err := database.DB.Where("customer_id = ?", customerID).Order("updated_at desc").First(&thread).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"thread": nil, "messages": []models.SupportMessage{}})
		return
	}
	var messages []models.SupportMessage
	database.DB.Where("thread_id = ?", thread.ID).Order("created_at asc").Find(&messages)
	c.JSON(http.StatusOK, gin.H{"thread": thread, "messages": messages})
}

// AdminListThreadsHandler handles GET /admin/support/threads. Enriched
// with customer identity and a per-thread message count/last-sender so the
// admin list can actually show who's waiting on a reply, rather than just
// a thread ID and a truncated snippet — every field here is a real lookup
// against the customer/message rows, nothing synthesized.
func AdminListThreadsHandler(c *gin.Context) {
	var threads []models.SupportThread
	database.DB.Order("updated_at desc").Find(&threads)

	type threadSummary struct {
		models.SupportThread
		LastMessage    string `json:"lastMessage"`
		LastSenderType string `json:"lastSenderType"`
		MessageCount   int64  `json:"messageCount"`
		CustomerName   string `json:"customerName"`
		CustomerMobile string `json:"customerMobile"`
		AwaitingReply  bool   `json:"awaitingReply"`
	}
	out := make([]threadSummary, 0, len(threads))
	for _, t := range threads {
		var last models.SupportMessage
		preview := ""
		lastSender := ""
		if err := database.DB.Where("thread_id = ?", t.ID).Order("created_at desc").First(&last).Error; err == nil {
			preview = last.Body
			lastSender = last.SenderType
		}

		var count int64
		database.DB.Model(&models.SupportMessage{}).Where("thread_id = ?", t.ID).Count(&count)

		var user models.User
		customerName := ""
		customerMobile := ""
		if err := database.DB.First(&user, "id = ?", t.CustomerID).Error; err == nil {
			customerMobile = user.Mobile
			var profile models.CustomerProfile
			if err := database.DB.Where("user_id = ?", t.CustomerID).First(&profile).Error; err == nil {
				customerName = strings.TrimSpace(profile.FirstName + " " + profile.LastName)
			}
		}

		out = append(out, threadSummary{
			SupportThread:  t,
			LastMessage:    preview,
			LastSenderType: lastSender,
			MessageCount:   count,
			CustomerName:   customerName,
			CustomerMobile: customerMobile,
			AwaitingReply:  t.Status == "open" && lastSender == models.SupportSenderCustomer,
		})
	}
	c.JSON(http.StatusOK, out)
}

// AdminThreadMessagesHandler handles GET /admin/support/threads/:id/messages.
// The returned "thread" object carries the same customerName/customerMobile
// enrichment as AdminListThreadsHandler, so the detail panel's header can
// show who the agent is actually talking to, not just a thread number.
func AdminThreadMessagesHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var thread models.SupportThread
	if err := database.DB.First(&thread, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "thread not found"})
		return
	}
	var messages []models.SupportMessage
	database.DB.Where("thread_id = ?", thread.ID).Order("created_at asc").Find(&messages)

	var user models.User
	customerName := ""
	customerMobile := ""
	if err := database.DB.First(&user, "id = ?", thread.CustomerID).Error; err == nil {
		customerMobile = user.Mobile
		var profile models.CustomerProfile
		if err := database.DB.Where("user_id = ?", thread.CustomerID).First(&profile).Error; err == nil {
			customerName = strings.TrimSpace(profile.FirstName + " " + profile.LastName)
		}
	}

	threadOut := gin.H{
		"id": thread.ID, "customerId": thread.CustomerID, "status": thread.Status,
		"createdAt": thread.CreatedAt, "updatedAt": thread.UpdatedAt,
		"customerName": customerName, "customerMobile": customerMobile,
	}
	c.JSON(http.StatusOK, gin.H{"thread": threadOut, "messages": messages})
}

// AdminReplyHandler handles POST /admin/support/threads/:id/messages
func AdminReplyHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var thread models.SupportThread
	if err := database.DB.First(&thread, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "thread not found"})
		return
	}
	var body struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	msg := models.SupportMessage{
		ThreadID:   thread.ID,
		SenderType: models.SupportSenderAdmin,
		SenderRef:  c.GetString("admin_username"),
		Body:       body.Body,
	}
	if err := database.DB.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to send reply"})
		return
	}
	database.DB.Model(&thread).Update("updated_at", msg.CreatedAt)
	c.JSON(http.StatusCreated, msg)
}

// AdminCloseThreadHandler handles POST /admin/support/threads/:id/close
func AdminCloseThreadHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var thread models.SupportThread
	if err := database.DB.First(&thread, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "thread not found"})
		return
	}
	thread.Status = models.SupportThreadClosed
	database.DB.Save(&thread)
	c.JSON(http.StatusOK, thread)
}
