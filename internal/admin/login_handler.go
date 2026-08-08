package admin

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// LoginRequest is intentionally minimal — username/password against a
// static .env credential. Swapping this for a real admin_users table later
// only requires changing the body of LoginHandler (look up + bcrypt compare
// instead of the env comparison below); the request/response shapes and the
// rest of the API stay the same.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func LoginHandler(c *gin.Context) {

	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request",
		})
		return
	}

	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminUsername == "" || adminPassword == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Admin credentials not configured",
		})
		return
	}

	if req.Username != adminUsername || req.Password != adminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Invalid username or password",
		})
		return
	}

	token, err := GenerateToken(req.Username)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to generate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"admin": gin.H{
			"username": req.Username,
		},
	})
}
