package middleware

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

const adminRole = "admin"

// 檢查是否有admin權限，沒有則中止請求
func CheckAdminPermissionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("Role")
		if !exists {
			log.Println("無法取得Role")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "錯誤",
			})
			c.Abort()
			return
		}
		if role != adminRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "沒有權限",
			})
			c.Abort()
			return
		}

		c.Next()
		return
	}
}
