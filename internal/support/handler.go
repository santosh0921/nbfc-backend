package support

import (
	"net/http"
	"strconv"

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

// AdminListThreadsHandler handles GET /admin/support/threads
func AdminListThreadsHandler(c *gin.Context) {
	var threads []models.SupportThread
	database.DB.Order("updated_at desc").Find(&threads)

	type threadSummary struct {
		models.SupportThread
		LastMessage string `json:"lastMessage"`
	}
	out := make([]threadSummary, 0, len(threads))
	for _, t := range threads {
		var last models.SupportMessage
		preview := ""
		if err := database.DB.Where("thread_id = ?", t.ID).Order("created_at desc").First(&last).Error; err == nil {
			preview = last.Body
		}
		out = append(out, threadSummary{SupportThread: t, LastMessage: preview})
	}
	c.JSON(http.StatusOK, out)
}

// AdminThreadMessagesHandler handles GET /admin/support/threads/:id/messages
func AdminThreadMessagesHandler(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var thread models.SupportThread
	if err := database.DB.First(&thread, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "thread not found"})
		return
	}
	var messages []models.SupportMessage
	database.DB.Where("thread_id = ?", thread.ID).Order("created_at asc").Find(&messages)
	c.JSON(http.StatusOK, gin.H{"thread": thread, "messages": messages})
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
