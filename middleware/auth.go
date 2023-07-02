package middleware

import (
	"Backend/jwt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"strings"
)

func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token == "" {
			c.Header("Authorization", "")
			c.Next()
			return
		}

		//如Token不合法或錯誤則回傳空Authorization
		userID, role, err := jwt.VerifyToken(&token, db)
		if err != nil {
			log.Printf("無法驗證Token: %v\n", err)
			c.Header("Authorization", "")
			c.Next()
			return
		}

		c.Header("Authorization", authHeader)
		c.Set("Token", token)
		c.Set("UserID", userID)
		c.Set("Role", role)
		c.Next()
		return
	}
}
