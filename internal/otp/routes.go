package otp

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.Engine) {

	auth := router.Group("/auth")

	auth.POST("/send-otp", SendOTPHandler)
	auth.POST("/verify-otp", VerifyOTPHandler)
}